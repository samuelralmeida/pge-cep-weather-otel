package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func (h *Handler) WeatherHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	carrier := propagation.HeaderCarrier(r.Header)
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	_, span := h.Tracer.Start(ctx, "weather-handler")
	defer span.End()

	cep := chi.URLParam(r, "cep")

	cepInfo, err := getLocationData(cep)
	if err != nil {
		log.Println("getLocationData:", err)
		http.Error(w, "error to get location data in viacep api", http.StatusInternalServerError)
		return
	}

	if cepInfo == nil {
		log.Println("cep not found")
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	weatherInfo, err := getWeatherData(cepInfo.City, h.WeatherApiKey)
	if err != nil {
		log.Println("getWeatherData:", err)
		http.Error(w, "error to get weather data in weather api", http.StatusInternalServerError)
		return
	}

	render.Status(r, 200)
	render.Render(w, r, weatherInfo)
}

type cepInfo struct {
	Cep          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
}

func getLocationData(cep string) (*cepInfo, error) {
	type respData struct {
		Cep         string `json:"cep"`
		Logradouro  string `json:"logradouro"`
		Complemento string `json:"complemento"`
		Unidade     string `json:"unidade"`
		Bairro      string `json:"bairro"`
		Localidade  string `json:"localidade"`
		Uf          string `json:"uf"`
		Ibge        string `json:"ibge"`
		Gia         string `json:"gia"`
		Ddd         string `json:"ddd"`
		Siafi       string `json:"siafi"`
		Err         string `json:"erro"`
	}

	var data respData

	err := request(context.Background(), fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep), &data)
	if err != nil {
		return nil, fmt.Errorf("error requesting via cep: %w", err)
	}

	if data.Err != "" {
		return nil, nil
	}

	resp := &cepInfo{
		Cep:          data.Cep,
		State:        data.Uf,
		City:         data.Localidade,
		Neighborhood: data.Bairro,
		Street:       data.Logradouro,
	}

	return resp, err
}

type WeatherInfo struct {
	City       string  `json:"city"`
	Kelvin     float64 `json:"temp_K"`
	Celsius    float64 `json:"temp_C"`
	Fahrenheit float64 `json:"temp_F"`
}

// só para implementar a interface do Render do chi
func (wi *WeatherInfo) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func getWeatherData(location string, weatherApiKey string) (*WeatherInfo, error) {
	type respData struct {
		Current struct {
			TempC float64 `json:"temp_c"`
			TempF float64 `json:"temp_f"`
		} `json:"current"`
	}

	var data respData

	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no", weatherApiKey, url.QueryEscape(location))

	err := request(context.Background(), url, &data)
	if err != nil {
		return nil, fmt.Errorf("error requesting weather api: %w", err)
	}

	resp := &WeatherInfo{
		City:    location,
		Celsius: data.Current.TempC,
		// api devolve temperatura em Fahrenheit, não precisa calcular
		Fahrenheit: data.Current.TempF,
		Kelvin:     data.Current.TempC + 273,
	}

	return resp, err
}

func request(ctx context.Context, url string, data any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("error to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error to do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error to read body: %w", err)
	}

	err = json.Unmarshal(body, data)
	if err != nil {
		return fmt.Errorf("error to unmarshal body: %w", err)
	}

	return nil
}
