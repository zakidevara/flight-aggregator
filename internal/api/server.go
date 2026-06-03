package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/zakidevara/bookcabin-assessment/internal/filter"
	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/service"
)

// Server adapts HTTP requests to the transport-agnostic service layer.
type Server struct {
	svc *service.Service
}

func NewServer(svc *service.Service) *Server { return &Server{svc: svc} }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", s.handleSearch)
	return mux
}

// GET /search?origin=CGK&destination=DPS&date=2025-12-15
//
//	[&passengers=1&cabin=economy&sort=duration_asc]
//	[&min_price=&max_price=&max_stops=&max_duration=&airlines=GA,JT]
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "only GET is supported")
		return
	}
	q := r.URL.Query()

	origin := strings.ToUpper(strings.TrimSpace(q.Get("origin")))
	dest := strings.ToUpper(strings.TrimSpace(q.Get("destination")))
	date := strings.TrimSpace(q.Get("date"))
	if origin == "" || dest == "" || date == "" {
		writeError(w, http.StatusBadRequest, "origin, destination and date are required")
		return
	}

	passengers := 1
	if v := q.Get("passengers"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "passengers must be a positive integer")
			return
		}
		passengers = n
	}

	cabin := q.Get("cabin")
	if cabin == "" {
		cabin = "economy"
	}

	sortKey := filter.SortKey(q.Get("sort"))
	switch sortKey {
	case "", filter.SortPriceAsc, filter.SortPriceDesc, filter.SortDurationAsc,
		filter.SortDurationDesc, filter.SortDepartAsc, filter.SortArriveAsc:
	default:
		writeError(w, http.StatusBadRequest, "invalid sort key: "+string(sortKey))
		return
	}

	opts, err := parseFilters(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := service.Query{
		Request: model.SearchRequest{
			Origin:        origin,
			Destination:   dest,
			DepartureDate: date,
			Passengers:    passengers,
			CabinClass:    cabin,
		},
		Filters: opts,
		Sort:    sortKey,
	}

	// r.Context() ties the search to the client connection: if the caller hangs
	// up, the context cancels and in-flight provider calls stop.
	resp := s.svc.Search(r.Context(), query)
	writeJSON(w, http.StatusOK, resp)
}

func parseFilters(q url.Values) (filter.Options, error) {
	var o filter.Options
	if v, ok, err := qInt64(q, "min_price"); err != nil {
		return o, err
	} else if ok {
		o.MinPrice = &v
	}
	if v, ok, err := qInt64(q, "max_price"); err != nil {
		return o, err
	} else if ok {
		o.MaxPrice = &v
	}
	if v, ok, err := qInt(q, "max_stops"); err != nil {
		return o, err
	} else if ok {
		o.MaxStops = &v
	}
	if v, ok, err := qInt(q, "max_duration"); err != nil {
		return o, err
	} else if ok {
		o.MaxDurationMinutes = &v
	}
	if v := q.Get("airlines"); v != "" {
		o.Airlines = strings.Split(v, ",")
	}
	return o, nil
}

func qInt64(q url.Values, key string) (int64, bool, error) {
	v := q.Get(key)
	if v == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false, &paramError{key}
	}
	return n, true, nil
}

func qInt(q url.Values, key string) (int, bool, error) {
	v := q.Get(key)
	if v == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false, &paramError{key}
	}
	return n, true, nil
}

type paramError struct{ key string }

func (e *paramError) Error() string { return e.key + " must be an integer" }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
