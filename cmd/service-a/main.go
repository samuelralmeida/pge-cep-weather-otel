package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type config struct {
	port        string
	serviceBUrl string
}

var conf config

func init() {
	port := os.Getenv("SERVICE_A_PORT")
	serviceBUrl := os.Getenv("SERVICE_B_URL")

	conf = config{
		port:        port,
		serviceBUrl: serviceBUrl,
	}
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(60 * time.Second))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("route does not exist"))
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("method is not valid"))
	})

	r.Post("/", weatherHandler)

	log.Println("listening on port", conf.port)
	http.ListenAndServe(":"+conf.port, r)
}

func weatherHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Cep string `json:"cep"`
	}{}
	err := render.DecodeJSON(r.Body, &data)
	if err != nil {
		log.Println(err)
		http.Error(w, "invalid body request", http.StatusBadRequest)
		return
	}

	if !isValidCep(data.Cep) {
		log.Println("cep inválido:", data.Cep)
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	url := fmt.Sprintf("%s/weather/%s", conf.serviceBUrl, data.Cep)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to create external request", http.StatusInternalServerError)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to call external request", http.StatusInternalServerError)
		return
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to read external request", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != 200 {
		http.Error(w, string(bytes), resp.StatusCode)
		return
	}

	var result WeatherInfo

	err = json.Unmarshal(bytes, &result)
	if err != nil {
		log.Println(err)
		http.Error(w, "error to unmarshal body", http.StatusInternalServerError)
		return
	}

	render.Status(r, 200)
	render.Render(w, r, &result)
}

func isValidCep(cep string) bool {
	re := regexp.MustCompile(`^\d{8}$`)
	return re.MatchString(cep)
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
