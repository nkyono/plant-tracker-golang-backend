package awsplants

//package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

// PlantInfo contains plant information. PlantID is primary key; scientific is sort key
type PlantInfo struct {
	PlantID    int
	Common     string
	Scientific string
}

// OccurrenceInfo contains information about occurrences. OccurrenceID is primary; PlantID is sort
type OccurrenceInfo struct {
	OccurrenceID string
	Date         string
	Accuracy     float64
	Latitude     float64
	Longitude    float64
	PlantID      int
}

// GetPlants queries the database and returns list of PlantInfo
func GetPlants(svc *dynamodb.DynamoDB, vals ...map[string][]string) (*[]PlantInfo, int, error) {
	// Note: occurrence is spelt wrong
	var filt expression.ConditionBuilder

	if len(vals) <= 1 && len(vals[0]) == 0 {
		filt = expression.Name("PlantID").GreaterThan(expression.Value(-1))
	} else {
		for _, val := range vals {
			id, prs := val["common"]
			if prs {
				filt = expression.Name("Common").Equal(expression.Value(id[0]))
			}
			id, prs = val["scientific"]
			if prs {
				filt = expression.Name("Scientific").Equal(expression.Value(id[0]))
			}
			id, prs = val["id"]
			if prs {
				idInt, err := strconv.Atoi(id[0])
				if err != nil {
					fmt.Println("Failed int to string conversion")
					fmt.Println((err.Error()))
					return nil, -1, err
				}
				filt = expression.Name("PlantID").Equal(expression.Value(idInt))
			}
		}
	}

	proj := expression.NamesList(
		expression.Name("PlantID"),
		expression.Name("Common"),
		expression.Name("Scientific"),
	)

	expr, err := expression.NewBuilder().WithFilter(filt).WithProjection(proj).Build()
	if err != nil {
		fmt.Println("Got error building expression:")
		fmt.Println(err.Error())
		return nil, -1, err
	}

	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Plants"),
	}

	result, err := svc.Scan(params)
	if err != nil {
		fmt.Println("Query API call failed:")
		fmt.Println((err.Error()))
		return nil, -1, err
	}

	numItems := 0
	resultItems := make([]PlantInfo, 0)
	for _, i := range result.Items {
		item := PlantInfo{}
		err = dynamodbattribute.UnmarshalMap(i, &item)

		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			return nil, -1, err
		}

		numItems++
		//fmt.Printf("%+v\n", item)
		resultItems = append(resultItems, item)
	}
	//fmt.Printf("Found %d items\n", numItems)
	//fmt.Println("-----------------")
	/*
		for _, item := range resultItems {
			fmt.Printf("%+v\n", item)
		}
		fmt.Println("-----------------")
	*/
	return &resultItems, numItems, nil
}

// GetOccurrences queries database and returns list of OccurrenceInfo
func GetOccurrences(svc *dynamodb.DynamoDB, vals map[string][]string) (*[]OccurrenceInfo, int, error) {
	var filt expression.ConditionBuilder

	// parse query to get a plant id, if no id specified default to -1 (all plants)
	valID, prs := vals["id"]
	var id int
	var err error
	if prs {
		var err error
		id, err = strconv.Atoi(valID[0])
		if err != nil {
			id = -1
		}
		if id != -1 {
			filt = expression.Name("PlantID").Equal(expression.Value(id))
		} else {
			filt = expression.Name("PlantID").GreaterThan(expression.Value(id))
		}
	}
	if !prs || err != nil {
		id = -1
	}
	if id != -1 {
		filt = expression.Name("PlantID").Equal(expression.Value(id))
	} else {
		filt = expression.Name("PlantID").GreaterThan(expression.Value(id))
	}
	// parse query for beginning date. if none specified default to 1970/01/01
	// note: dates stored as strings
	d1, prs := vals["datefrom"]
	var dateFrom time.Time
	var err1 error
	if prs {
		dateFrom, err1 = time.Parse("20060102", d1[0])
	}
	if !prs || err1 != nil {
		dateFrom = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	d2, prs := vals["dateto"]
	var dateTo time.Time
	var err2 error
	if prs {
		dateTo, err2 = time.Parse("20060102", d2[0])
	}
	if !prs || err2 != nil {
		dateTo = time.Now()
		dateTo = dateTo.Add(24 * time.Hour)
	}

	// can only filter out one variable
	// going to stick with id
	/*
		if err1 == nil && err2 == nil {
			// was going to use the stuff below, but I realize that the defualt would do so not needed
			//dateFrom, _ = time.Parse("2006-01-02", "1970-01-01")
			fmt.Println(dateFrom.Format("2006-01-02"))
			fmt.Println(dateTo.Format("2006-01-02"))
			filt = expression.Name("Date").Between(expression.Value(dateFrom.Format("2006-01-02")), expression.Value(dateTo.Format("2006-01-02")))
		}
	*/
	// parse query for accuracy. if none specified default to 0.0

	/*
		// commented out because you cannot query on multiple attributes at once
		// due to the ids, only date or accuracy or id can be queried at a time
		// can retrieve items and then filter them
		acc, err := strconv.ParseFloat(vals["acc"][0], 64)
		if err == nil {
			filt = expression.Name("Accuracy").GreaterThan(expression.Value(acc / 100.0))
		} else {
			fmt.Println("acc fail")
		}
	*/
	a, prs := vals["acc"]
	var acc float64
	acc = 0.0
	if prs {
		acc, _ = strconv.ParseFloat(a[0], 64)
		acc = acc / 100.0
	}

	proj := expression.NamesList(
		expression.Name("OccurrenceID"),
		expression.Name("Date"),
		expression.Name("Accuracy"),
		expression.Name("Latitude"),
		expression.Name("Longitude"),
		expression.Name("PlantID"),
	)
	expr, err := expression.NewBuilder().WithFilter(filt).WithProjection(proj).Build()
	if err != nil {
		fmt.Println("Got error building expression:")
		fmt.Println(err.Error())
		return nil, -1, err
	}

	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Occurrences"),
	}

	result, err := svc.Scan(params)
	if err != nil {
		fmt.Println("Query API call failed:")
		fmt.Println((err.Error()))
		return nil, -1, err
	}

	numItems := 0

	resultItems := make([]OccurrenceInfo, 0)

	for _, i := range result.Items {
		item := OccurrenceInfo{}
		err = dynamodbattribute.UnmarshalMap(i, &item)

		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			return nil, -1, err
		}
		if item.Accuracy > acc && item.Date > dateFrom.Format("2006-01-02") && item.Date < dateTo.Format("2006-01-02") {
			numItems++
			resultItems = append(resultItems, item)
		}

		//fmt.Printf("%+v\n", item)
	}

	/*fmt.Printf("Found %d items\n", numItems)
	fmt.Println("-----------------")
	fmt.Println()
	*/
	/*
		for _, item := range resultItems {
			fmt.Printf("%+v\n", item)
		}
		fmt.Println("-----------------")
	*/
	return &resultItems, numItems, nil
}

func getItems() []OccurrenceInfo {
	raw, err := ioutil.ReadFile("./miscItems.json")
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var items []OccurrenceInfo
	json.Unmarshal(raw, &items)
	return items
}

func addItems(svc *dynamodb.DynamoDB) {
	// get items from json file
	items := getItems()
	// get number of items from database
	vals := make(map[string][]string)
	vals["id"] = []string{"-1"}
	// _, numItems, _ := GetOccurrences(svc, vals)
	tableName := "Occurrences"

	// add new items
	for _, item := range items {
		time.Sleep(time.Nanosecond * 1)
		sha := sha1.New()

		sha.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		s := sha.Sum(nil)
		hs := fmt.Sprintf("%x", s)

		item.OccurrenceID = hs

		s1 := rand.NewSource(time.Now().UnixNano() + 30)
		r1 := rand.New(s1)

		dateItem, _ := time.Parse("2006-01-02", item.Date)
		item.Date = dateItem.Add(time.Hour * time.Duration(24*r1.Intn(50))).Format("2006-01-02")
		item.Accuracy = r1.Float64()
		item.Latitude = item.Latitude + r1.Float64()
		item.Longitude = item.Longitude + r1.Float64()

		av, err := dynamodbattribute.MarshalMap(item)
		if err != nil {
			fmt.Println("Got error marshalling map:")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		// Create item in table Movies
		input := &dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String(tableName),
		}

		_, err = svc.PutItem(input)
		if err != nil {
			fmt.Println("Got error calling PutItem:")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println("Successfully added " + item.OccurrenceID + " to table " + tableName)
	}
}

// struct used for storing credentials
/*
type awsCreds struct {
	User string `json:"user"`
	Akey string `json:"access_key"`
	Skey string `json:"secret"`
}

func main() {
	c, err := ioutil.ReadFile("../creds.json")
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

	addItems(svc)
	// GetPlants(svc)
	// GetOccurrences(svc, 1)
}
*/
