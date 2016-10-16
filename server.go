/*
 *
 */
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	//deal with environment variables
	"github.com/caarlos0/env"

	//official GCM library
	"github.com/google/go-gcm"

	//URL router and dispatcher
	"github.com/gorilla/mux"

	//handle CORS requests
	"github.com/rs/cors"
)

type ServerConfig struct {
	// port the server run on. Default is 5000
	ServerPort int `env:"SERVER_PORT" envDefault:"5000"`

	// API key (from Firebase Cloud Console)
	ApiKey string `env:"FCM_API_KEY,required"`

	// GCM sender ID (from Firebase Cloud Console)
	SenderId string `env:"FCM_SENDER_ID,required"`

	//Debug mode: print logging
	Debug bool `env:"DEBUG_MODE" envDefault:"false"`
}

type MessageStruct struct {
	Protocol string          `json:"protocol"`
	Message  json.RawMessage `json:"message"`
}

type HttpError struct {
	Error string `json:"error"`
}

var (
	serverConfig ServerConfig
	port         int
	apiKey       string
	senderId     string
	debug        bool
)

func sendJSON(writer http.ResponseWriter, obj interface{}) {
	json.NewEncoder(writer).Encode(obj)
}

func sendUnprocessableEntity(writer http.ResponseWriter, err error) error {
	writer.Header().Set("Content-Type", "application/json; charset=UTF-8")
	writer.WriteHeader(http.StatusNotAcceptable)
	return json.NewEncoder(writer).Encode(err)
}

func SendOkResponse(writer http.ResponseWriter, res interface{}) {
	log.Printf("Response: %+v", res)
	writer.WriteHeader(http.StatusOK)
	sendJSON(writer, res)
}

func SendMessageSendError(writer http.ResponseWriter, sendErr error) {
	log.Println("Message send error: %+v", sendErr)
	writer.WriteHeader(http.StatusInternalServerError)
	sendJSON(writer, sendErr)
}

// Handle request to send a new message.
func SendMessage(writer http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(io.LimitReader(req.Body, 1048576))

	if err != nil {
		log.Fatal(err)
	}
	if err := req.Body.Close(); err != nil {
		log.Fatal(err)
	}

	// Decode the passed body into the struct.
	var message MessageStruct
	if err := json.Unmarshal(body, &message); err != nil {
		sendUnprocessableEntity(writer, err)
		return
	}

	protocol := strings.ToLower(message.Protocol)

	if protocol == "http" {
		// Send HTTP message
		var httpMsg gcm.HttpMessage
		if err := json.Unmarshal(message.Message, &httpMsg); err != nil {
			log.Println("Message Unmarshal error: %+v", err)
			sendUnprocessableEntity(writer, err)
			return
		}

		res, sendErr := gcm.SendHttp(apiKey, httpMsg)
		if sendErr != nil {
			SendMessageSendError(writer, sendErr)
		} else {
			SendOkResponse(writer, res)
		}
	} else if protocol == "xmpp" {
		// Send XMPP message
		var xmppMsg gcm.XmppMessage
		if err := json.Unmarshal(message.Message, &xmppMsg); err != nil {
			log.Println("Message Unmarshal error: %+v", err)
			sendUnprocessableEntity(writer, err)
			return
		}

		res, _, sendErr := gcm.SendXmpp(senderId, apiKey, xmppMsg)
		if sendErr != nil {
			SendMessageSendError(writer, sendErr)
		} else {
			SendOkResponse(writer, res)
		}
	} else {
		// Error
		writer.WriteHeader(http.StatusBadRequest)
		sendJSON(writer, &HttpError{"protocol should be HTTP or XMPP only."})
	}
}

// Route handler for the server
func Handler() http.Handler {
	router := mux.NewRouter()

	// POST /message
	// Send a new message
	router.HandleFunc("/message", SendMessage).Methods("POST")

	corsConfig := cors.New(cors.Options{
		AllowCredentials: true,
	})
	return corsConfig.Handler(router)
}

func main() {

	serverConfig := ServerConfig{}

	configErr := env.Parse(&serverConfig)
	if configErr != nil {
		log.Fatal(fmt.Sprintf("%+v", configErr))
	}

	port = serverConfig.ServerPort
	apiKey = serverConfig.ApiKey
	senderId = serverConfig.SenderId
	debug = serverConfig.Debug

	gcm.DebugMode = debug

	//Start the server
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), Handler())
	if err != nil {
		log.Fatal("ListenAndServe: " + err.Error())
	}
	log.Println(fmt.Sprintf("Started - serving at port %v", port))
}
