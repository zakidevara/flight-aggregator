package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "time/tzdata" // embed IANA tz DB so LoadLocation works on any OS

	"github.com/zakidevara/bookcabin-assessment/internal/api"
	"github.com/zakidevara/bookcabin-assessment/internal/filter"
	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/money"
	"github.com/zakidevara/bookcabin-assessment/internal/provider"
	"github.com/zakidevara/bookcabin-assessment/internal/service"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	demo := flag.Bool("demo", false, "run a one-off sample search and exit")
	flag.Parse()

	providers := []provider.Provider{
		provider.Garuda{},
		provider.LionAir{},
		provider.BatikAir{},
		provider.AirAsia{},
	}
	svc := service.New(providers, 2*time.Second)

	if *demo {
		runDemo(svc)
		return
	}

	srv := &http.Server{
		Addr:         *addr,
		Handler:      api.NewServer(svc).Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Cancel the context on Ctrl-C / SIGTERM, then drain in-flight requests.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Printf("listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func runDemo(svc *service.Service) {
	q := service.Query{
		Request: model.SearchRequest{
			Origin: "CGK", Destination: "DPS", DepartureDate: "2025-12-15",
			Passengers: 1, CabinClass: "economy",
		},
		Sort: filter.SortBestValue,
	}
	resp := svc.Search(context.Background(), q)
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
	if len(resp.Flights) > 0 {
		b := resp.Flights[0]
		fmt.Printf("\nTop pick: %s %s -> %s, %s, %s %s, %d stop(s)\n",
			b.FlightNumber, b.Departure.Airport, b.Arrival.Airport,
			b.Duration.Formatted, money.FormatIDR(b.Price.Amount), b.Price.Currency, b.Stops)
	}
}

// func main() {

// 	req := model.SearchRequest{
// 		Origin:        "CGK",
// 		Destination:   "DPS",
// 		DepartureDate: "2026-08-15",
// 		Passengers:    1,
// 		CabinClass:    "economy",
// 	}

// 	agg := aggregator.FetchAll(
// 		context.Background(),
// 		[]provider.Provider{provider.AirAsia{}, provider.LionAir{}, provider.Garuda{}, provider.BatikAir{}},
// 		req,
// 	)

// 	f := filter.Options{}

// 	flights := filter.Apply(agg.Flights, f)
// 	filter.Sort(flights, filter.SortDurationAsc)

// 	out, _ := json.MarshalIndent(flights, "", "  ")
// 	fmt.Println(string(out))

// 	// printAirline("AirAsia", provider.AirAsia{})
// 	// printAirline("LionAir", provider.LionAir{})
// }

// func printAirline(airline string, p provider.Provider) {

// 	req := model.SearchRequest{
// 		Origin:        "CGK",
// 		Destination:   "DPS",
// 		DepartureDate: "2026-08-15",
// 		Passengers:    1,
// 		CabinClass:    "economy",
// 	}

// 	fmt.Printf("--- %q ---\n", airline)

// 	flights, err := p.Search(context.Background(), req)
// 	if err != nil {
// 		fmt.Println("search failed:", err)
// 		return
// 	}

// 	out, _ := json.MarshalIndent(flights, "", "  ")
// 	fmt.Println(string(out))

// }
