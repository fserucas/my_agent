package main

import (
	"context"
	"fmt"
	"log"
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
			Description: "Retrieves the list of countries which can retrieve the capitals and the population number.",
		},
		getListOfCountries,
	)

	populationTool, err := functiontool.New(
		functiontool.Config{
			Name: "get_population_country",
			Description: "Retrieves the population number for a given country.",
		},
		getPopulationCountry,
	)

	agent, err := llmagent.New(llmagent.Config{
		Name:        "capital_agent",
		Model:       model,
		// Description: "Answers user questions about the capital city of a given country.",
		Description: "Answers user questions about the capital city and the population number of a given country.",
		// Instruction: "You are an agent that provides the capital city of a country... (previous instruction text)",
		Instruction: "You are an agent that provides the capital city and the population of a country... (previous instruction text)",
		Tools:       []tool.Tool{
			capitalTool,
			populationTool,
			listOfCountriesTool,
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
