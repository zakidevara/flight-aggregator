package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zakidevara/bookcabin-assessment/internal/aggregator"
	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/provider"
)

func main() {

	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2026-08-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	res := aggregator.FetchAll(
		context.Background(),
		[]provider.Provider{provider.AirAsia{}, provider.LionAir{}, provider.Garuda{}, provider.BatikAir{}},
		req,
	)

	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))

	// printAirline("AirAsia", provider.AirAsia{})
	// printAirline("LionAir", provider.LionAir{})
}

func printAirline(airline string, p provider.Provider) {

	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2026-08-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	fmt.Printf("--- %q ---\n", airline)

	flights, err := p.Search(context.Background(), req)
	if err != nil {
		fmt.Println("search failed:", err)
		return
	}

	out, _ := json.MarshalIndent(flights, "", "  ")
	fmt.Println(string(out))

}
