package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"

	Plants "./awsplants"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// ShiftPath gets the head and tail/rest of the url
func ShiftPath(p string) (head, tail string) {
	p = path.Clean("/" + p)
	i := strings.Index(p[1:], "/") + 1
	if i <= 0 {
		return p[1:], "/"
	}
	return p[1:i], p[i:]
}

// things seems unnecessary. I think only occurrences is need with a query string
// GET will return the occurrences
// PUT will add an occurrence

// App represents the base of the URL path
type App struct {
	OccurrencesHandler *OccurrencesHandler
	SpeciesHandler     *SpeciesHandler
	AwsConnection      *dynamodb.DynamoDB
}

// OccurrencesHandler represents the part of URL that specifies that we are getting the occurrences
type OccurrencesHandler struct {
	OccurrenceHandler *OccurrenceHandler
	AwsConnection     *dynamodb.DynamoDB
}

// SpeciesHandler represents the part of URL that specifies the species
type SpeciesHandler struct {
	AwsConnection *dynamodb.DynamoDB
}

// OccurrenceHandler represents the part of URL that deals with a specific occurrence
type OccurrenceHandler struct {
	AwsConnection *dynamodb.DynamoDB
}

// query string after ../occurrences?...
// id=_&datefrom=_&dateto=_

// App ServeHTTP handler deals with routing ./occurrences... or ./species...
func (handler *App) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Access-Control-Allow-Origin", "*")
	var head string
	head, _ = ShiftPath(req.URL.Path)

	//fmt.Printf("Handling: %s\n", head)

	switch head {
	case "species":
		handler.SpeciesHandler.AwsConnection = handler.AwsConnection
		handler.SpeciesHandler.ServeHTTP(res, req)
	case "occurrences":
		handler.OccurrencesHandler.AwsConnection = handler.AwsConnection
		handler.OccurrencesHandler.ServeHTTP(res, req)
	default:
		http.Error(res, "Not Found", http.StatusNotFound)
	}
}

func (handler *SpeciesHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// var head string
	_, req.URL.Path = ShiftPath(req.URL.Path)

	// fmt.Printf("SpeciesHandler Handling: %s\n", head)

	// there should be a mapping of plant species/plant ids
	// i feel like the mapping should be stored elsewhere so that its not recomputed
	// species is a list of plantInfo struct

	switch req.Method {
	case "GET":
		if next, _ := ShiftPath(req.URL.Path); next != "" {
			http.Error(res, "Not Found", http.StatusNotFound)
			return
		}

		species, _, err := Plants.GetPlants(handler.AwsConnection, req.URL.Query())

		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		type Species []Plants.PlantInfo
		var body Species = *species
		/*
			for _, item := range *species {
				fmt.Fprintf(res, "%+v\n", item)
			}
		*/
		resBody, err := json.Marshal(body)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write(resBody)
	case "POST":
		http.Error(res, "POST not implemented", http.StatusMethodNotAllowed)
	default:
		http.Error(res, "Only GET and POST are allowed", http.StatusMethodNotAllowed)
	}
}

func checkValid(species []Plants.PlantInfo, id string) bool {
	for _, item := range species {
		if strconv.Itoa(item.PlantID) == id {
			return true
		}
	}
	return false
}

// ServeHTTP takes in the database connection and plant id then handles the .../occurrences/.. path
func (handler *OccurrencesHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// var head string
	_, req.URL.Path = ShiftPath(req.URL.Path)
	// fmt.Printf("%v\n", req.URL.Query())
	// fmt.Printf("OccurrencesHandler Handling: %s\n", head)

	switch req.Method {
	case "GET":
		//fmt.Fprintf(res, "Occurrences\n")
		if next, _ := ShiftPath(req.URL.Path); next != "" {
			http.Error(res, "Not Found", http.StatusNotFound)
			return
		}

		occurrences, _, err := Plants.GetOccurrences(handler.AwsConnection, req.URL.Query())

		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		/*
			for _, item := range *occurrences {
				fmt.Fprintf(res, "%+v\n", item)
			}
		*/
		type Occurrences []Plants.OccurrenceInfo
		var body Occurrences = *occurrences

		resBody, err := json.Marshal(body)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		res.Header().Set("Content-Type", "application/json")
		res.Write(resBody)
	case "POST":
		handler.OccurrenceHandler = new(OccurrenceHandler)
		handler.OccurrenceHandler.AwsConnection = handler.AwsConnection
		handler.OccurrenceHandler.ServeHTTP(res, req)
	default:
		http.Error(res, "Only GET and POST are allowed", http.StatusMethodNotAllowed)
	}
}

func (handler *OccurrenceHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	var head string
	head, req.URL.Path = ShiftPath(req.URL.Path)

	if head != "" {
		http.Error(res, "Not Found", http.StatusNotFound)
		return
	}

	var occur Plants.OccurrenceInfo

	err1 := json.NewDecoder(req.Body).Decode(&occur)
	fmt.Printf("%+v\n", occur)
	if err1 != nil {
		http.Error(res, err1.Error(), http.StatusBadRequest)
		return
	}

	err2 := Plants.AddItem(handler.AwsConnection, occur)
	if err2 != nil {
		http.Error(res, err2.Error(), http.StatusInternalServerError)
		return
	}
}

// struct used for storing credentials
type awsCreds struct {
	User string `json:"user"`
	Akey string `json:"access_key"`
	Skey string `json:"secret"`
}

func connectAws() *dynamodb.DynamoDB {
	c, err := ioutil.ReadFile("./creds.json")
	if err != nil {
		panic(err)
	}

	creds := awsCreds{}
	_ = json.Unmarshal([]byte(c), &creds)

	fmt.Println("------------------")
	fmt.Printf("%s\n", creds.User)
	fmt.Printf("%s\n", creds.Akey)
	fmt.Printf("%s\n", creds.Skey)
	fmt.Println("------------------")

	// create a new session that connects to aws with read credentials
	mySession := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("us-west-1"),
		Credentials: credentials.NewStaticCredentials(creds.Akey, creds.Skey, ""),
	}))
	svc := dynamodb.New(mySession)
	return svc
}

func main() {
	svc := connectAws()
	a := &App{
		OccurrencesHandler: new(OccurrencesHandler),
		SpeciesHandler:     new(SpeciesHandler),
		AwsConnection:      svc,
	}
	http.ListenAndServe(":8000", a)
}
