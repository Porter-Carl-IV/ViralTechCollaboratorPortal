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

    "github.com/jung-kurt/gofpdf"
    "github.com/jung-kurt/gofpdf/contrib/barcode"
    _"github.com/lib/pq"
    log "github.com/sirupsen/logrus"
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
  UserMessage Message `json:"userMessage"`
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
  Expandable bool `json:"expandable"`
  SpreadsheetConfig []Column `json:"spreadsheetConfig"`
}
type Column struct{
  ReadOnly bool `json:"readOnly"`
  Data string `json:"data"`
  Type string `json:"type"`
  Source []string  `json:"source"`
}
type Message struct{
  Subject string `json:"subject"`
  Message string `json:"message"`
  CurrentToken string `json:"currentToken"`
  CurrentPacID int `json:"currentPacID"`
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
const sqlGroupPackage =
`SELECT exists(
  SELECT 1
  FROM pims2.package
  WHERE package_group_id = $1 AND package_id = $2);`
const sqlAddMessage =
`UPDATE pims2.user
SET user_messages = $1
WHERE user_email = $2;`
const sqlGetMessages =
`SELECT user_messages
FROM pims2.user
WHERE user_email = $1;`


func main(){
  //Open database connection
  psqlInfo := fmt.Sprintf("host=%s port=%d user=%s " +
    "password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
  var dbErr error

  db,dbErr = sql.Open("postgres", psqlInfo)
  if dbErr != nil {
    log.Error(dbErr)
  }
  defer db.Close()

  dbErr = db.Ping()
  if dbErr != nil{
    log.Error(dbErr)
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
  http.HandleFunc( "/printQR/" , printQR )
  http.HandleFunc( "/addMessage/" , addMessage )
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
    log.Error(readErr)
  }

  //Parse parameters into struct
  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication and get user email
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
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
      log.Error(sendErr)
    }

    writer.Write( returnString )

    return
  }

  var packageCount int;

  groupID := getGroupId( userEmail )
  if( groupID == USER_NOT_IN_DATABASE ) {

    returnString, sendErr := json.Marshal("User not found")
    if sendErr != nil {
      log.Error( sendErr );
    }

    writer.Write( returnString )

    return
  }

    row := db.QueryRow(sqlPackageCountQuery, groupID)

    switch err := row.Scan(&packageCount); err {
      case sql.ErrNoRows:
        returnString, sendErr := json.Marshal("Group has no current packages")
        if sendErr != nil {
          log.Error( sendErr );
        }

        writer.Write( returnString )
        return
      case nil:
      default:
        log.Error(err)
      }

    returnData := make([]InitialReturn, packageCount)
    count := 0;

    rows, err := db.Query(sqlPackageQuery, groupID)
    if err != nil {
      log.Error(err)
    }
    defer rows.Close()

    for rows.Next() {
      err = rows.Scan(&returnData[count].PackageDate, &returnData[count].StepID, &returnData[count].PackageID, &returnData[count].ErrorCount )
      if err != nil {
        // handle this error
        log.Error(err)
      }
      returnData[count].PackageDate = strings.Replace(returnData[count].PackageDate, "T00:00:00Z" , "", 1)

      t, _ := time.Parse(layoutISO, returnData[count].PackageDate)
      returnData[count].PackageDate = t.Format(layoutUS)

      count++
    }
    // get any error encountered during iteration
    err = rows.Err()
    if err != nil {
      log.Error(err)
    }

    returnData = sortPackages( returnData )

    returnString, sendErr := json.Marshal( returnData )
    if sendErr != nil{
      log.Error(sendErr)
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
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
  }

  //Get config for user
  config := pullConfig(4)

  if( DEFAULT_MODE ){
    returnString, sendErr := json.Marshal("Success")
    if sendErr != nil {
      log.Error( sendErr )
    }

    writer.Write( returnString )
    return
  }

  formatErr := checkFormat( config , request.DataEntry )
  if formatErr != nil {
    returnError, jsErr := json.Marshal(formatErr.Error())
    if jsErr != nil {
      log.Error( jsErr )
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
      log.Error(marshalErr)
    }

    _, err := db.Exec( sqlSampleInsert, id , metaData )
    if err != nil {
      log.Error(err)
    }
  }

  _, stepErr := db.Exec( sqlSetStep, 2, request.PackageID )
  if stepErr != nil {
    log.Error(stepErr)
  }

  returnString, sendErr := json.Marshal("Success")
  if sendErr != nil {
    log.Error( sendErr )
  }

  writer.Write( returnString )
  return


}

//updatePackage will update the packages metadata
//Takes: JSON: Authentication Token, packageID, 2D Spreadsheet array
//Returns: Success message
func updatePackage( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
  }

  if( DEFAULT_MODE ) {
    returnString, sendErr := json.Marshal("Success")
    if sendErr != nil {
      log.Error( sendErr );
    }

    writer.Write( returnString )

    return
  }

  insertData, sendErr := json.Marshal( request.DataEntry )
  if sendErr != nil{
    log.Error(sendErr)
  }

  //Insert temp data into temp column
  _, err := db.Exec( sqlTempUpdate, insertData, request.PackageID )
  if err != nil {
    log.Error(err)
  }

  returnString, sendErr := json.Marshal("Package Saved Successfully")
  if sendErr != nil {
    log.Error( sendErr );
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
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
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
      log.Error(sendErr);
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
      log.Error(jsonErr)
    }
  }

  //If user has expandable privalage, check saved data to see if they
  //expanded it. If they did, add it to config
  if returnVal.Expandable {
    returnVal = checkExtraColumns( returnVal )
  }

  //Check Package Step ID
  var stepID int
  row = db.QueryRow( sqlStepQuery , returnVal.PackageID )
  dbErr = row.Scan( &stepID )
  if dbErr != nil{
    log.Error( dbErr )
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
    log.Error(sendErr)
  }

  //Write return JSON string into the response
  writer.Write( returnString )

}

func newPackage( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  groupID := getGroupId( userEmail )
  if( groupID == USER_NOT_IN_DATABASE ) {

    returnString, sendErr := json.Marshal("User not found")
    if sendErr != nil {
      log.Error( sendErr );
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
    log.Error(sendErr)
  }

  _, err := db.Exec( sqlTempUpdate, insertData, packageID )
  if err != nil {
    log.Error(err)
  }

  returnPacID, sendErr := json.Marshal(packageID)
  if sendErr != nil {
    log.Error( sendErr );
  }

  writer.Write( returnPacID )


}

func addMessage( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  var messages []Message
  var messageString string
  row := db.QueryRow( sqlGetMessages , userEmail )
  dbErr := row.Scan( &messageString )
  if dbErr == nil{
    jsonErr = json.Unmarshal([]byte(messageString), &messages)
    if jsonErr != nil {
      log.Error(jsonErr)
    }
  }

  messages = append( messages, request.UserMessage )

  insertData, sendErr := json.Marshal( messages )
  if sendErr != nil{
    log.Error(sendErr)
  }

  //Insert temp data into temp column
  _, err := db.Exec( sqlAddMessage, insertData, userEmail )
  if err != nil {
    log.Error(err)
  }

  returnString, sendErr := json.Marshal("Success")
  if sendErr != nil {
    log.Error( sendErr );
  }

  writer.Write( returnString )

}
func addTracking( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
  }

  _, stepErr := db.Exec( sqlSetTrackingNumber, request.TrackingNumber, request.PackageID )
  if stepErr != nil {
    log.Error(stepErr)
  } else {

    returnString, sendErr := json.Marshal("Success")
    if sendErr != nil {
      log.Error( sendErr )
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
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
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
    log.Error(err)
  }

  returnString, sendErr := json.Marshal("{\"ID on Submitted Tube\":\"" + strconv.Itoa(id[0]) + "\"}")
  if sendErr != nil {
    log.Error( sendErr );
  }

  writer.Write( returnString )
}

func checkErrors( writer http.ResponseWriter, r *http.Request ) {
  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
  }

  var errors string
  row := db.QueryRow( sqlGetErrors , request.PackageID )
  dbErr := row.Scan( &errors )
  if dbErr != nil{
    log.Error(dbErr)
  }

  writer.Write( []byte( errors ) );
}

func reserveIDs( numIDs int ) []int {
  ids := make( []int, numIDs )

  count := 0

  rows, err := db.Query(sqlReserveIDs, numIDs)
  if err != nil {
    // **TODO** handle this error better than this
    log.Error(err)
  }
  defer rows.Close()

  for rows.Next() {
    err = rows.Scan(&ids[count])
    if err != nil {
      // handle this error
      log.Error(err)
    }
    count++
  }
  // get any error encountered during iteration
  err = rows.Err()
  if err != nil {
    log.Error(err)
  }

  return ids;

}

func authPackage( groupID int, packageID int ) error {
  var exist bool

  //Query package where groupID and PackageID are the ones given
  row := db.QueryRow( sqlGroupPackage, groupID, packageID )
  dbErr := row.Scan( &exist )

  if dbErr != nil{
    log.Error(dbErr)
  }

  //If the record with packageID and groupID doesn't exist, return error
  if !exist {
    return errors.New("Package not associated with group")
  }

  return nil;
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
  var errorList []string
  errorCount := 1

  //Loop through spreadsheet
  for row := 0; row < len( spreadsheet ); row++ {
    for col := 0; col < len( config.SpreadsheetConfig ); col++ {
      value, present := spreadsheet[row][config.SpreadsheetConfig[col].Data]
      if !present {
        errorList = append(errorList, fmt.Sprintf("Error %d: %s column left blank for sample %s" , errorCount, config.SpreadsheetConfig[col].Data, spreadsheet[row]["ID on Submitted Tube"]))
        errorCount++
        continue
      }

      switch config.SpreadsheetConfig[col].Type {
      case "numeric":
        _, intErr := strconv.Atoi( value )
        _, floatErr := strconv.ParseFloat( value, 64 )

        if intErr != nil && floatErr != nil {
          errorList = append( errorList, fmt.Sprintf("Error %d: %s column expects numeric value for sample %s" , errorCount, config.SpreadsheetConfig[col].Data, spreadsheet[row]["ID on Submitted Tube"]))
          errorCount++
        }
      case "text":
      case "dropdown":
      }
    }
  }

  if len(errorList) != 0 {
    errorList = append( []string{ fmt.Sprintf( "Failed to submit package with %d errors, Please fix these errors:" , errorCount - 1 ) } , errorList... )
    return errors.New( strings.Join( errorList, "\n" ) )
  }

  return nil
}

func pullConfig( configID int ) Config {
  var config Config

  file, err := os.Open("config2.txt") // For read access.
  if err != nil {
     log.Error(err)
  }

  data := make([]byte, 5000)
  count, err2 := file.Read(data)
  if err2 != nil {
    log.Error(err2)
  }

  jsonErr := json.Unmarshal(data[:count], &config)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  return config;
}

func checkExtraColumns( config Spreadsheet ) Spreadsheet {

  for index := 0; index < len(config.Metadata); index++ {
    for key := range config.Metadata[index] {
      if !inData( config.SpreadsheetConfig, key ) {
        config.SpreadsheetConfig = append( config.SpreadsheetConfig, Column{ false, key, "text", nil } )
        config.ColHeaders = append( config.ColHeaders, key )
      }
    }
  }

  return config;
}

func inData( config []Column, data string ) bool {

  for index := 0; index < len( config ); index++ {
    if config[index].Data == data {
      return true;
    }
  }

  return false;
}

func getColHeaders( config Config ) []string {
  //Calculate the # of column headers in spreadsheet
  var colHeaders []string

  //Iterate through the pool and non-pool maps in the config, adding the names
  //of the column headers to the string slice ColHeaders
  for index := 0; index < len( config.SpreadsheetConfig ); index++ {
    colHeaders = append( colHeaders, config.SpreadsheetConfig[index].Data )
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
    log.Error(err)
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

func printQR( writer http.ResponseWriter, r *http.Request ) {

  body, readErr := ioutil.ReadAll(r.Body)
  if readErr != nil {
    log.Error(readErr)
  }

  var request Param
  jsonErr := json.Unmarshal(body, &request)
  if jsonErr != nil {
    log.Error(jsonErr)
  }

  //Check Authentication
  userEmail, authErr := authenticate( request.AuthToken )
  if authErr != nil {
    log.Error(authErr)
  }

  //Make sure package belongs to user
  pacErr := authPackage( getGroupId( userEmail ), request.PackageID )
  if pacErr != nil {
    log.Error(pacErr)
  }

  pdf := makeBlankPDFTemplate()

  var spreadsheetMapString string
  var spreadsheetMap []map[string]string
  row := db.QueryRow( sqlGetMetaData , request.PackageID )
  dbErr := row.Scan( &spreadsheetMapString )
  if dbErr == nil{
    jsonErr = json.Unmarshal([]byte(spreadsheetMapString), &spreadsheetMap)
    if jsonErr != nil {
      log.Error(jsonErr)
    }
  }

  var sampIDs []int
  for index := 0; index < len(spreadsheetMap); index++ {
    tempInt, err := strconv.Atoi(spreadsheetMap[index]["ID on Submitted Tube"])
    if err!=nil{
      log.Error(err)
    }
    sampIDs = append( sampIDs, tempInt)
  }

  pdf = makeBarcodePDFFromID( pdf, "pims2", sampIDs )

  saveErr := pdf.OutputFileAndClose( fmt.Sprintf("QRCodes/%d/Package_%d.pdf" , getGroupId( userEmail ) , request.PackageID ) )
  if saveErr != nil {
    log.Error(saveErr)
  }

  returnString, sendErr := json.Marshal( fmt.Sprintf("QRCodes/%d/Package_%d.pdf" , getGroupId( userEmail ) , request.PackageID ) )
  if sendErr != nil {
    log.Error( sendErr );
  }

  writer.Write( returnString )

  return

}

//Method written by TGen North
func makeBlankPDFTemplate() *gofpdf.Fpdf{
	pdf := gofpdf.New("P", "in", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 1)
	return pdf
}

//Method written by TGen North
func makeBarcodePDFFromID(pdf *gofpdf.Fpdf, schema string, sampleIDs []int) *gofpdf.Fpdf {
	//pass in an array of sample ids and build up the barcodes from the database
	//pass in the pdf existing pdf to append to and return the pointer  to make it clear that is the pdf we have modified

	s := gofpdf.SizeType{1, 1}// this is because our barcodes are 1x1

	for idx := 0; idx < len(sampleIDs); idx++ {
		log.Debug("sub_sample ID: ", sampleIDs[idx])
		subsampleLocation := -1
		sampleProject := -1
		subsampleParent := -1
		subsampleSpecies := ""
		sampleType := ""

		db.QueryRow(`SELECT sub_sample_location, sub_sample_parent, project_id, species_code, sub_sample_data->>'sample_type'
			FROM `+schema+`.sub_sample
				JOIN `+schema+`.sample ON sub_sample_parent = sample.global_id
				JOIN `+schema+`.project ON sample.sample_data ->> 'project_name' = project_name
				JOIN `+schema+`.species ON sub_sample_data ->>'species_name' = species_name
				WHERE sub_sample.global_id = $1
				`, (int)(sampleIDs[idx])).Scan(&subsampleLocation, &subsampleParent, &sampleProject, &subsampleSpecies, &sampleType)

		log.Debug("SL: ", subsampleLocation, " SP: ", subsampleParent, " SPRO: ", sampleProject, " SS: ", subsampleSpecies)

		//use the location given and the getLocationAndParent function to get the locations we need.
		//this is only valid when the selected thing is a rack
		log.Debug("Subsample location: ", subsampleLocation)
		if subsampleLocation <= 0 {
			log.Error("Invalid location given for : ", sampleIDs[idx])
		}
		cellName, boxID := getLocationAndParent(schema, subsampleLocation)
		log.Debug("Box ID: ", boxID)
		boxName, rackID := getLocationAndParent(schema, boxID)
		log.Debug("Rack ID: ", rackID)
		//	freezerName, _ := getLocationAndParent(pgtx, schema, rackID)
		log.Debug(sampleType)
		pdf.AddPageFormat("L", s) //create a new page to put the label on
		var opt gofpdf.ImageOptions
		opt.ImageType = "png"
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(100, 100, 100)
		//.CellFormat(width, height, text, 1 for newline 0 for not, letter abbreviation for text format C = Center, false, 0, "")
		key := barcode.RegisterDataMatrix(pdf, "tg@"+strconv.Itoa(sampleIDs[idx])) //this is the barcode
		barcode.Barcode(pdf, key, .35, 0, .3, .5, false)                           //size and postition of barcode
//		pdf.ImageOptions("cap.png", 0, 0, .2, .5, false, opt, 0, "")               //the cap sticker on the left
		if sampleType == "DNA" {
			pdf.CellFormat(.95, .3, sampleType, "0", 1, "R", false, 0, "")             //this is the DNA flag in the top right
		}else{
			pdf.CellFormat(.95, .3, "", "0", 1, "R", false, 0, "")             //this is for sapcing if no DNA flag
		}
		pdf.Ln(.2) //makes space for rest of text
		pdf.CellFormat(1, .1, "Sample #: "+strconv.Itoa(subsampleParent), "1", 1, "C", false, 0, "") //this is the parent#
		pdf.SetFont("Arial", "B", 5)
		pdf.CellFormat(1, .1, sampleType +" #: "+strconv.Itoa(sampleIDs[idx]), "1", 1, "C", false, 0, "") //this is the TG number (global_id) the unique identifier
		pdf.CellFormat(1, .1, "Box: "+boxName, "1", 1, "C", false, 0, "")              // this is the box and cell
		pdf.CellFormat(1, .1, "Cell: "+cellName, "1", 1, "C", false, 0, "")            // this is the box and cell
		pdf.CellFormat(.5, .1, strconv.Itoa(sampleProject), "1", 0, "C", false, 0, "") //this is the project code
		pdf.CellFormat(.5, .1, subsampleSpecies, "1", 1, "C", false, 0, "")            // this is the species code

	}
	return pdf
}

//Method written by TGen North
func getLocationAndParent( schema string, locID int) (string, int) { //this gets the location name and parent location id for the locaion id passed in
	mylocation := ""
	parentid := 0
	if err := db.QueryRow(`SELECT location_name, location_parent FROM `+schema+`.location WHERE location.location_id  = $1`, locID).Scan(&mylocation, &parentid); err != nil {
		log.Error("locID: ", locID)
		log.Error(err)
	}
	return mylocation, parentid

}
