package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/go-chi/render"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	Tracer            trace.Tracer
	WeatherServiceUrl string
	WeatherApiKey     string
}

func (h *Handler) InputHandler(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), carrier)
	ctx, span := h.Tracer.Start(ctx, "input-handler")
	defer span.End()

	cep, err := decodeCepFromBodyRequest(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, "invalid body request", http.StatusBadRequest)
		return
	}

	if !isValidCep(cep) {
		log.Println("cep inválido:", cep)
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	url := fmt.Sprintf("%s/weather/%s", h.WeatherServiceUrl, cep)
	responseData, statusCode, err := requestWeatherData(ctx, url)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to request weather data", http.StatusInternalServerError)
		return
	}

	if statusCode != 200 {
		http.Error(w, string(responseData), statusCode)
		return
	}

	var result TemperatureRespInfo
	err = json.Unmarshal(responseData, &result)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to unmarshal body", http.StatusInternalServerError)
		return
	}

	render.Status(r, 200)
	render.Render(w, r, &result)
}

func decodeCepFromBodyRequest(body io.ReadCloser) (string, error) {
	data := struct {
		Cep string `json:"cep"`
	}{}
	err := render.DecodeJSON(body, &data)
	return data.Cep, err
}

func isValidCep(cep string) bool {
	re := regexp.MustCompile(`^\d{8}$`)
	return re.MatchString(cep)
}

func requestWeatherData(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error to do request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error to read response body: %w", err)
	}

	return bodyBytes, resp.StatusCode, nil
}

type TemperatureRespInfo struct {
	City       string  `json:"city"`
	Kelvin     float64 `json:"temp_K"`
	Celsius    float64 `json:"temp_C"`
	Fahrenheit float64 `json:"temp_F"`
}

// só para implementar a interface do Render do chi
func (t *TemperatureRespInfo) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
