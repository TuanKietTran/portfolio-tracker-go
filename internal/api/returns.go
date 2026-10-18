package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/shopspring/decimal"
)

func (h *PortfolioHandlers) returns(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	query := r.URL.Query()
	from, err := strconv.ParseInt(query.Get("from"), 10, 64)
	if err != nil || from <= 0 {
		writePortfolioError(w, errors.New("positive Unix second from required"))
		return
	}
	to, err := strconv.ParseInt(query.Get("to"), 10, 64)
	if err != nil || to <= 0 {
		writePortfolioError(w, errors.New("positive Unix second to required"))
		return
	}
	age, err := strconv.ParseInt(query.Get("max_age"), 10, 64)
	if err != nil || age < 0 {
		writePortfolioError(w, errors.New("nonnegative max_age required"))
		return
	}
	options := command.ReturnOptions{Method: query.Get("method"), MaxAge: age}
	if options.Method == "MWR" {
		options.IRR = command.DefaultIRROptions()
		for _, item := range []struct {
			name   string
			target *decimal.Decimal
		}{{"rate_tolerance", &options.IRR.RateTolerance}, {"npv_tolerance", &options.IRR.NPVTolerance}} {
			if query.Has(item.name) {
				raw := query.Get(item.name)
				if len(raw) > 64 {
					writePortfolioError(w, errors.New("tolerance exceeds64 characters"))
					return
				}
				value, err := decimal.NewFromString(raw)
				if err != nil {
					writePortfolioError(w, errors.New("invalid decimal tolerance"))
					return
				}
				*item.target = value
			}
		}
		if query.Has("max_iterations") {
			iterations, err := strconv.Atoi(query.Get("max_iterations"))
			if err != nil {
				writePortfolioError(w, errors.New("invalid max_iterations"))
				return
			}
			options.IRR.MaxIterations = iterations
		}
	} else if query.Has("rate_tolerance") || query.Has("npv_tolerance") || query.Has("max_iterations") {
		writePortfolioError(w, errors.New("solver options require method MWR"))
		return
	}
	result, err := h.service.Returns(r.Context(), id, query.Get("currency"), time.Unix(from, 0), time.Unix(to, 0), options)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 200, result)
}
