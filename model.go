//Package containing server side model of data for TGen North collaborator data entry website
package main

import (
    "database/sql"
    "fmt"
    "encoding/json"
    "strings"
    "net/http"
    "errors"
    "io/ioutil"
    "os"
    "time"
    "strconv"

    _"github.com/lib/pq"
)

const (
  host       = "localhost"
  port       = 5432
  user       = "capstone"
  password   = "admin"
  dbname     = "pmi"
  DEFAULT_MODE = false
  USER_NOT_IN_DATABASE = -1
  layoutISO = "2006-01-02"
  layoutUS  = "01-02-2006"
)

type TokenData struct {
	Iss string `json:"iss"`
	Azp string `json:"azp"`
	Aud string `json:"aud"`
	Sub string `json:"sub"`
	Email string `json:"email"`
	EmailVerified string `json:"email_verified"`
	AtHash string `json:"at_hash"`
	Name string `json:"name"`
	Picture string `json:"picture"`
	GivenName string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Locale string `json:"locale"`
	Iat string `json:"iat"`
	Exp string `json:"exp"`
	Jti string `json:"jti"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}
type Param struct{
  AuthToken string `json:"token"`
  PackageID int `json:"packageID"`
	DataEntry []map[string]string `json:"spreadsheet"`
  SampleNumber int `json:"sampleNumber"`
  TrackingNumber string `json:"trackingNumber"`
}

type Spreadsheet struct{
  Expandable bool `json:"expandable"`
  PackageID int `json:"packageID"`
  SpreadsheetConfig []Column `json:"spreadsheetConfig"`
  ColHeaders []string `json:"columnHeaders"`
  Metadata []map[string]string `json:"metadata"`
}

type InitialReturn struct{
  PackageID int `json:"packageID"`
  StepID int `json:"stepID"`
  PackageDate string `json:"packageDate"`
  ErrorCount int `json:"errorCount"`
}

type Config struct{
  ConfigID int `json:"configID"`
  Expandable bool `json:"expandable"`
  PoolLoc string `json:"poolLoc"`
  SpreadsheetConfig []Column `json:"spreadsheetConfig"`
  NonPool []map[string]string `json:"nonPool"`
  Pool []map[string]string `json:pool`
}
type Column struct{
  ReadOnly bool `json:"readOnly"`
  Data string `json:"data"`
  Type string `json:"type"`
  Source []string  `json:"source"`
}

var db *sql.DB
const sqlPackageCountQuery =
`SELECT count(*)
FROM pims2.package
WHERE package_group_id = $1;`
const sqlPackageQuery =
`SELECT package_create_date, package_step_id, package_id, package_error_count
FROM pims2.package
WHERE package_group_id =$1;`
const sqlTempUpdate =
`UPDATE pims2.package
SET package_temp_metadata = $1
WHERE package_id = $2;`
const sqlGetMetaData =
`SELECT package_temp_metadata
FROM pims2.package
WHERE package_id = $1;`
const sqlStepQuery =
`SELECT package_step_id
FROM pims2.package
WHERE package_id = $1;`
const sqlGroupIDQuery =
`SELECT group_id
FROM pims2.user_group_bridge
WHERE user_id = (SELECT user_id FROM pims2.user WHERE user_email=$1);`
const sqlPackageInsert =
`INSERT INTO pims2.package( package_group_id, package_create_date, package_step_id, package_error_count )
VALUES( $1, $2, $3, 0 ) RETURNING package_id;`
const sqlReserveIDs =
`SELECT nextval('pims2.global_id_seq') FROM generate_series(1,$1);`
const sqlSampleInsert =
`INSERT INTO pims2.sample( global_id , sample_data )
VALUES( $1 , $2 )`
const sqlSetStep =
`UPDATE pims2.package
SET package_step_id = $1
WHERE package_id = $2;`
const sqlSetTrackingNumber =
`UPDATE pims2.package
SET package_tracking_id = $1
WHERE package_id = $2;`
const sqlGetErrors =
`SELECT package_errors
FROM pims2.package
WHERE package_id = $1;`

func main(){
  //Open database connection
  psqlInfo := fmt.Sprintf("host=%s port=%d user=%s " +
    "password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
  var dbErr error

  db,dbErr = sql.Open("postgres", psqlInfo)
  if dbErr != nil {
    panic(dbErr)
  }
  defer db.Close()

  dbErr = db.Ping()
  if dbErr != nil{
    panic(dbErr)
  }

  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request ){
    http.ServeFile(w, r, r.URL.Path[1:])
  })
  http.HandleFunc( "/initialize/", initialize )
  http.HandleFunc( "/insertPackage/", insertPackage )
  http.HandleFunc( "/updatePackage/", updatePackage, )
  http.HandleFunc( "/generateSpreadsheet/", generateSpreadsheet )
  http.HandleFunc( "/newPackage/" , newPackage )
  http.HandleFunc( "/newSample/" , newSample )
  http.HandleFunc( "/addTracking/" , addTracking )
  http.HandleFunc( "/checkErrors/" , checkErrors )
  http.ListenAndServe( ":8080", nil )

}

//Initialize searches database for all packages belonging to a user.
//Once the packages have been found they are classified by sent or unsent.
//Classified packages are then put into a JSON object and returned to caller.
//Takes: JSON: Authentication Token
//Returns: JSON: Array of Tuples with first value being packageID and second
//being boolean denoting if package is editable.
func initialize( writer http.ResponseWriter, r *http.Request ) {

  //Read parameters
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  //Parse parameters into struct
  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication and get user email
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  //config := pullConfig( 4 );

  //DEFAULT MODE
  //Returns default values for testing purposes
  if( DEFAULT_MODE ) {
    var returnVal = []InitialReturn {
      InitialReturn{
        PackageID: 034,
        StepID: 4,
        PackageDate: "08-28-2019",
      },
      InitialReturn{
        PackageID: 274,
        StepID: 2,
        PackageDate: "10-20-2019",
      },
      InitialReturn{
        PackageID: 938,
        StepID: 1,
        PackageDate: "01-23-2020",
      },
    }

    returnString, sendErr := json.Marshal( returnVal )
    if sendErr != nil{
      panic(sendErr)
    }

    writer.Write( returnString )

    return
  }

  var packageCount int;

  groupID := getGroupId( userEmail )
  if( groupID == USER_NOT_IN_DATABASE ) {

    returnString, sendErr := json.Marshal("User not found")
    if sendErr != nil {
      panic( sendErr );
    }

    writer.Write( returnString )

    return
  }

    row := db.QueryRow(sqlPackageCountQuery, groupID)

    switch err := row.Scan(&packageCount); err {
      case sql.ErrNoRows:
        returnString, sendErr := json.Marshal("Group has no current packages")
        if sendErr != nil {
          panic( sendErr );
        }

        writer.Write( returnString )
        return
      case nil:
      default:
        panic(err)
      }

    returnData := make([]InitialReturn, packageCount)
    count := 0;

    rows, err := db.Query(sqlPackageQuery, groupID)
    if err != nil {
      panic(err)
    }
    defer rows.Close()

    for rows.Next() {
      err = rows.Scan(&returnData[count].PackageDate, &returnData[count].StepID, &returnData[count].PackageID, &returnData[count].ErrorCount )
      if err != nil {
        // handle this error
        panic(err)
      }
      returnData[count].PackageDate = strings.Replace(returnData[count].PackageDate, "T00:00:00Z" , "", 1)

      t, _ := time.Parse(layoutISO, returnData[count].PackageDate)
      returnData[count].PackageDate = t.Format(layoutUS)

      count++
    }
    // get any error encountered during iteration
    err = rows.Err()
    if err != nil {
      panic(err)
    }

    returnData = sortPackages( returnData )

    returnString, sendErr := json.Marshal( returnData )
    if sendErr != nil{
      panic(sendErr)
    }

    writer.Write( returnString )

}

//insertPackage checks formatting of spreadsheet data and if it's correct it
//inserts it into the database.
//Takes: JSON: Authentication token, packageID, 2D Spreadsheet array
//Returns: Success message, .pdf manifest and QR codes
func insertPackage( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  //Get config for user
  config := pullConfig(4)

  if( DEFAULT_MODE ){
    returnString, sendErr := json.Marshal("Sucess")
    if sendErr != nil {
      panic( sendErr )
    }

    writer.Write( returnString )
    return
  }

  formatErr := checkFormat( config , request.DataEntry )
  if formatErr != nil {
    returnError, jsErr := json.Marshal(formatErr)
    if jsErr != nil {
      panic( jsErr )
    }

    writer.Write( returnError )
    return
  }

  for index := 0; index < len( request.DataEntry ); index++ {
    id, _ := strconv.Atoi( ( request.DataEntry[index]["ID on Submitted Tube"] ))

    request.DataEntry[index]["package_id"] = strconv.Itoa(request.PackageID)

    delete( request.DataEntry[index] , "ID on Submitted Tube" )

    metaData, marshalErr := json.Marshal( request.DataEntry[index] )
    if marshalErr != nil {
      panic(marshalErr)
    }

    _, err := db.Exec( sqlSampleInsert, id , metaData )
    if err != nil {
      panic(err)
    }
  }

  _, stepErr := db.Exec( sqlSetStep, 2, request.PackageID )
  if stepErr != nil {
    panic(stepErr)
  }

  returnString, sendErr := json.Marshal("Sucess")
  if sendErr != nil {
    panic( sendErr )
  }
  //**TODO**
  //Return .pdf version of shipping manifest
  writer.Write( returnString )
  return


}

//updatePackage will update the packages metadata
//Takes: JSON: Authentication Token, packageID, 2D Spreadsheet array
//Returns: Success message
func updatePackage( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  if( DEFAULT_MODE ) {
    returnString, sendErr := json.Marshal("Sucess")
    if sendErr != nil {
      panic( sendErr );
    }

    writer.Write( returnString )

    return
  }

  insertData, sendErr := json.Marshal( request.DataEntry )
  if sendErr != nil{
    panic(sendErr)
  }

  //Insert temp data into temp column
  _, err := db.Exec( sqlTempUpdate, insertData, request.PackageID )
  if err != nil {
    panic(err)
  }

  returnString, sendErr := json.Marshal("Package Saved Successfully")
  if sendErr != nil {
    panic( sendErr );
  }

  writer.Write( returnString )

}

//generateSpreadsheet will return the formatting for the spreadsheet for the
//user.
//Takes: JSON: Authentication Token, packageID (If null, new spreadsheet)
//Returns: JSON: Spreadsheet format/data
func generateSpreadsheet( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  //Get config for user
  config := pullConfig(4)

  colHeaders := getColHeaders( config )

  var returnVal Spreadsheet
  returnVal.ColHeaders = colHeaders
  returnVal.Expandable = config.Expandable
  returnVal.PackageID = request.PackageID
  returnVal.SpreadsheetConfig = config.SpreadsheetConfig


  if( DEFAULT_MODE ) {
    fmt.Println(request.PackageID)
    fmt.Println(request.AuthToken)
    //Create the return struct and add col headers to it
    returnVal.Metadata = []map[string]string {
      { "Project Name":"MoldovaSeq-Yale" , "Id on submitted tube":"TG252151" ,
        "Sample Name":"111-20301-18" , "Species Name" : "Mycobacterium tuberculosis" ,
        "Sample Type" : "DNA" , "Country of Isolation" : "Moldova" ,
        "Year of Sample Collection" : "2019" , "Template Input Type" : "isolate DNA" ,
        "Final DNA Input for WGS (ug)" : "1" , "DNA QC method" : "Qubit HS dsDNA" ,
        "Desired Coverage(x)" : "100" },
        { "Project Name":"MoldovaSeq-Yale" , "Id on submitted tube":"TG252153" ,
           "Sample Name":"111-20109-18" , "Species Name" : "Mycobacterium tuberculosis" ,
            "Sample Type" : "DNA" , "Country of Isolation" : "Moldova" ,
            "Year of Sample Collection" : "2019" , "Template Input Type" :
            "isolate DNA" , "Final DNA Input for WGS (ug)" : "1" ,
            "DNA QC method" : "Qubit HS dsDNA" , "Desired Coverage(x)" : "100" },
        { "Project Name":"MoldovaSeq-Yale" , "Id on submitted tube":"TG252155" ,
          "Sample Name":"111-19944-18" , "Species Name" : "Mycobacterium tuberculosis" ,
          "Sample Type" : "DNA" , "Country of Isolation" : "Moldova" ,
          "Year of Sample Collection" : "2019" , "Template Input Type" : "isolate DNA" ,
          "Final DNA Input for WGS (ug)" : "1" , "DNA QC method" :
          "Qubit HS dsDNA" , "Desired Coverage(x)" : "100" } }

    //Parse return struct into JSON string
    returnString, sendErr := json.Marshal( returnVal );
    if sendErr != nil{
      panic(sendErr);
    }

    //Write return JSON string into the response
    writer.Write( returnString );

    return
  }

  var spreadsheetMap string
  row := db.QueryRow( sqlGetMetaData , returnVal.PackageID )
  dbErr := row.Scan( &spreadsheetMap )
  if dbErr == nil{
    jsonErr = json.Unmarshal([]byte(spreadsheetMap), &returnVal.Metadata)
    if jsonErr != nil {
      panic(jsonErr)
    }
  }

  //Check Package Step ID
  var stepID int
  row = db.QueryRow( sqlStepQuery , returnVal.PackageID )
  dbErr = row.Scan( &stepID )
  if dbErr != nil{
    panic( dbErr )
  }

  //If package step is greater than 1, set all columns to read only
  if stepID > 1 {
    for index := range returnVal.SpreadsheetConfig {
      returnVal.SpreadsheetConfig[index].ReadOnly = true
    }
  }


  //Parse return struct into JSON string
  returnString, sendErr := json.Marshal( returnVal )
  if sendErr != nil{
    panic(sendErr)
  }

  //Write return JSON string into the response
  writer.Write( returnString )

}

func newPackage( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  groupID := getGroupId( userEmail )
  if( groupID == USER_NOT_IN_DATABASE ) {

    returnString, sendErr := json.Marshal("User not found")
    if sendErr != nil {
      panic( sendErr );
    }

    writer.Write( returnString )

    return
  }
  packageID := createNewPackage( groupID )

  ids := reserveIDs( request.SampleNumber )

  insertDataMap := make( []map[string]string, len(ids) )

  for index := 0; index < len(ids) ; index++ {
    insertDataMap[index] = make( map[string]string )
    insertDataMap[index]["ID on Submitted Tube"] = strconv.Itoa(ids[index])
  }

  insertData, sendErr := json.Marshal( insertDataMap )
  if sendErr != nil{
    panic(sendErr)
  }

  _, err := db.Exec( sqlTempUpdate, insertData, packageID )
  if err != nil {
    panic(err)
  }

  returnPacID, sendErr := json.Marshal(packageID)
  if sendErr != nil {
    panic( sendErr );
  }

  writer.Write( returnPacID )


}

func addTracking( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  _, stepErr := db.Exec( sqlSetTrackingNumber, request.TrackingNumber, request.PackageID )
  if stepErr != nil {
    panic(stepErr)
  } else {

    returnString, sendErr := json.Marshal("Success")
    if sendErr != nil {
      panic( sendErr )
    }
    //**TODO**
    //Return .pdf version of shipping manifest
    writer.Write( returnString )
    return
  }
}

func newSample( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  id := reserveIDs( 1 )

  var spreadsheetMap string
  row := db.QueryRow( sqlGetMetaData , request.PackageID )

  dbErr := row.Scan( &spreadsheetMap )
  if dbErr == nil{
    //Append new map to map array with string replace
    spreadsheetMap = strings.Replace( spreadsheetMap , "]" , ",{\"ID on Submitted Tube\":\"" + strconv.Itoa(id[0]) + "\"}]" , 1 )
  }

  //Insert temp data into temp column
  _, err := db.Exec( sqlTempUpdate, spreadsheetMap, request.PackageID )
  if err != nil {
    panic(err)
  }

  returnString, sendErr := json.Marshal("{\"ID on Submitted Tube\":\"" + strconv.Itoa(id[0]) + "\"}")
  if sendErr != nil {
    panic( sendErr );
  }

  writer.Write( returnString )
}

func checkErrors( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    panic(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    panic(jsonErr)
  }

  //Check Authentication
  _, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    panic(authErr)
  }

  var errors string
  row := db.QueryRow( sqlGetErrors , request.PackageID )
  dbErr := row.Scan( &errors )
  if dbErr != nil{
    panic(dbErr)
  }

  writer.Write( []byte( errors ) );
}

func reserveIDs( numIDs int ) []int {
  ids := make( []int, numIDs )

  count := 0

  rows, err := db.Query(sqlReserveIDs, numIDs)
  if err != nil {
    // **TODO** handle this error better than this
    panic(err)
  }
  defer rows.Close()

  for rows.Next() {
    err = rows.Scan(&ids[count])
    if err != nil {
      // handle this error
      panic(err)
    }
    count++
  }
  // get any error encountered during iteration
  err = rows.Err()
  if err != nil {
    panic(err)
  }

  return ids;

}

//authenticate checks token to make sure it's valid.
//Takes: Token
//Returns: error, null if valid
func authenticate( token string ) (string, error) {
	resp, err := http.Get( "https://oauth2.googleapis.com/tokeninfo?id_token=" + token )
	if err != nil {
		return "", errors.New("Token not authenticated, please try again")
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Token not authenticated, please try again")
	}
	//log.Println(string(body)) prints Body, which is a token

	// Unmarshal the data into the struct
	var tokenData TokenData
	err = json.Unmarshal(body, &tokenData)
	if err != nil {
		fmt.Println(err)
		return "", errors.New("Error Unmarshalling data")
	}

	//Prints entire Token Data
	return tokenData.Email, nil

}

func checkFormat( config Config, spreadsheet []map[string]string ) error {

  //Loop through spreadsheet
  for row := 0; row < len( spreadsheet ); row++ {
    for col := 0; col < len( spreadsheet[row] ); col++ {

    }
  }

  return nil
}

func pullConfig( configID int ) Config {
  var config Config

  file, err := os.Open("config2.txt") // For read access.
  if err != nil {
     panic(err)
  }

  data := make([]byte, 5000)
  count, err2 := file.Read(data)
  if err2 != nil {
    panic(err2)
  }

  jsonErr := json.Unmarshal(data[:count], &config)
  if jsonErr != nil {
    panic(jsonErr)
  }

  return config;
}

func getColHeaders( config Config ) []string {
  //Calculate the # of column headers in spreadsheet
  headers := len(config.NonPool) + len(config.Pool)
  colHeaders := make( []string, headers )

  //Iterate through the pool and non-pool maps in the config, adding the names
  //of the column headers to the string slice ColHeaders
  for index := 0; index < headers; index++ {
    if( index >= len( config.NonPool ) ) {
      colHeaders[index] = config.Pool[index - len( config.NonPool )]["Name"]
    } else {
      colHeaders[index] = config.NonPool[index]["Spreadsheet_Name"]
    }
  }

  return colHeaders
}

func getGroupId( userEmail string ) int {
  var groupID int

  rows := db.QueryRow(sqlGroupIDQuery, userEmail)

  err := rows.Scan(&groupID)
  if err != nil{
    return USER_NOT_IN_DATABASE
  }

  return groupID
}

func createNewPackage( groupID int ) int {
  var packageID int;
  currentTime := time.Now()

  err := db.QueryRow( sqlPackageInsert, groupID , currentTime.Format("2006-01-02") , 1 ).Scan(&packageID)
  if err != nil {
    panic(err)
  }

  return packageID;
}

func sortPackages( packages []InitialReturn ) []InitialReturn {
  var tempPackage InitialReturn
  sorted := false;

  for !sorted {
    sorted = true;
    for index := 0; index < len(packages) - 1; index++ {
      tCurrent, _ := time.Parse( layoutUS, packages[index].PackageDate )
      tNext, _ := time.Parse( layoutUS, packages[index + 1].PackageDate )

      if tCurrent.Before(tNext) {
        tempPackage = packages[index + 1]
        packages[index + 1] = packages[index]
        packages[index] = tempPackage
        sorted = false;
      }
    }
  }

  return packages
}
