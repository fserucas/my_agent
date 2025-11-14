package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher/adk"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/server/restapi/services"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	_ "google.golang.org/adk/tool/geminitool"
	"google.golang.org/genai"
)

func main() {

	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Define a tool function
	type getCapitalCityArgs struct {
		Country string `json:"country" jsonschema:"The country to get the capital of."`
	}

	type getPopulationCountryArgs struct {
		Country string `json:"country" jsonschema:"The country to get the population of."`
	}

	type getTemperatureArgs struct {
		City string `json:"city" jsonschema:"The capital city to get the temperature of."`
	}

	var cityCoordinates = map[string]struct{ Lat, Lon float64 }{
		"paris":  {Lat: 48.85, Lon: 2.35},
		"tokyo":  {Lat: 35.68, Lon: 139.69},
		"ottawa": {Lat: 45.42, Lon: -75.69},
		"lisbon": {Lat: 38.72, Lon: -9.14},
	}

	type MeteoResponse struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
		} `json:"current"`
	}

	getTemperature := func(ctx tool.Context, args getTemperatureArgs) map[string]any {
		city := strings.ToLower(args.City)
		coords, ok := cityCoordinates[city]
		if !ok {
			return map[string]any{"result": fmt.Sprintf("Sorry, I don't have coordinates for %s.", args.City)}
		}

		url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m", coords.Lat, coords.Lon)

		resp, err := http.Get(url)
		if err != nil {
			return map[string]any{"result": fmt.Sprintf("Failed to call weather API: %v", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return map[string]any{"result": fmt.Sprintf("Weather API returned status: %s", resp.Status)}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return map[string]any{"result": fmt.Sprintf("Failed to read API response: %v", err)}
		}

		var weatherData MeteoResponse
		if err := json.Unmarshal(body, &weatherData); err != nil {
			return map[string]any{"result": fmt.Sprintf("Failed to parse weather JSON: %v", err)}
		}

		result := fmt.Sprintf("The current temperature in %s is %.1f°C.", args.City, weatherData.Current.Temperature)
		return map[string]any{"result": result}
	}

	getCapitalCity := func(ctx tool.Context, args getCapitalCityArgs) map[string]any {
		// Replace with actual logic (e.g., API call, database lookup)
		capitals := map[string]string{
			"france":   "Paris",
			"japan":    "Tokyo",
			"canada":   "Ottawa",
			"portugal": "Lisbon"}
		capital, ok := capitals[strings.ToLower(args.Country)]
		if !ok {
			return map[string]any{"result": fmt.Sprintf("Sorry, I don't know the capital of %s.", args.Country)}
		}
		return map[string]any{"result": capital}
	}

	getPopulationCountry := func(ctx tool.Context, args getPopulationCountryArgs) map[string]any {
		populations := map[string]string{
			"france":   "66 milion",
			"japan":    "123 million",
			"canada":   "39 million",
			"portugal": "10 million"}

		population, ok := populations[strings.ToLower(args.Country)]
		if !ok {
			return map[string]any{"result": fmt.Sprintf("Sorry, I don't know the population number of %s,", args.Country)}
		}
		return map[string]any{"result": population}
	}

	// Add the tool to the agent
	capitalTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_capital_city",
			Description: "Retrieves the capital city for a given country.",
		},
		getCapitalCity,
	)

	if err != nil {
		log.Fatal(err)
	}

	getListOfCountries := func(ctx tool.Context, args any) []string {
		countries := []string{
			"france",
			"japan",
			"canada",
			"portugal",
		}
		return countries
	}

	listOfCountriesTool, err := functiontool.New(
		functiontool.Config{
			Name: "get_list_of_countries",
			//		Description: "Retrieves the list of countries which can retrieve the capitals and the population number.",
			Description: "Retrieves the list of countries for which we can provide the capital, population, and temperature.",
		},
		getListOfCountries,
	)

	populationTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_population_country",
			Description: "Retrieves the population number for a given country.",
		},
		getPopulationCountry,
	)

	temperatureTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_temperature_for_capital",
			Description: "Retrieves the current temperature for a given capital city.",
		},
		getTemperature,
	)
	if err != nil {
		log.Fatal(err)
	}

	agent, err := llmagent.New(llmagent.Config{
		Name:  "capital_agent",
		Model: model,
		// // Description: "Answers user questions about the capital city of a given country.",
		// Description: "Answers user questions about the capital city and the population number of a given country.",
		// // Instruction: "You are an agent that provides the capital city of a country... (previous instruction text)",
		// Instruction: "You are an agent that provides the capital city and the population of a country... (previous instruction text)",
		Description: "Answers user questions about the capital city, population, and current temperature of a given country/city.",
		Instruction: "You are an agent that provides the capital city, population, and current temperature of a country. Use the available tools to find the information.",
		Tools: []tool.Tool{
			capitalTool,
			populationTool,
			listOfCountriesTool,
			temperatureTool,
		},
	})

	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &adk.Config{
		AgentLoader: services.NewSingleAgentLoader(agent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
