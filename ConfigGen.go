package main

import (
    "fmt"
    "encoding/json"
    "os"
)

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

func main(){
  var returnVal = Config {
      ConfigID: 001,
      Expandable: true,
      PoolLoc: "pims2.sample.json",
      SpreadsheetConfig: []Column{
        {
          ReadOnly : true,
          Data : "ID on Submitted Tube",
          Type : "numeric",
        },
        {
          Data : "Project Name",
          Type : "text",
        },
        {
          Data : "Sample Name",
          Type : "text",
        },
        {
          Data : "Species Name",
          Type : "text",
        },
        {
          Data : "Sample Type",
          Type : "text",
        },
        {
          Data : "Country of Isolation",
          Type : "text",
        },
        {
          Data : "Year of Sample Collection",
          Type : "numeric",
        },
        {
          Data : "Template Input Type",
          Type : "text",
        },
        {
          Data : "Final DNA Input for WGS (ug)",
          Type : "numeric",
        },
        {
          Data : "DNA QC method",
          Type : "dropdown",
          Source : []string{ "yellow" , "red" , "orange" , "green" , "blue" , "gray" , "black" , "white" },
        },
        {
          Data : "Desired Coverage (x)",
          Type : "numeric",
        },
      },
      NonPool: []map[string]string{
        {
          "Table_Name" : "pims.sample",
          "Key_Name" : "global_id",
          "Type" : "Key",
          "Spreadsheet_Name" : "ID on Submitted Tube",
        },
      },
      Pool: []map[string]string{
        {
          "Name" : "Project Name",
          "Type" : "String",
          "Mandatory" : "true",
        },
        {
          "Name" : "Sample Name" ,
          "Type" : "String",
          "Mandatory" : "true",
        },
        {
          "Name" : "Species Name" ,
          "Type" : "String",
          "Mandatory" : "true",
        },
        {
          "Name" : "Sample Type" ,
          "Type" : "String",
          "Mandatory" : "true",
        },
        {
          "Name" : "Country of Isolation" ,
          "Type" : "String",
          "Mandatory" : "true",
        },
        {
          "Name" : "Year of Sample Collection" ,
          "Type" : "Int",
          "Mandatory" : "false",
        },
        {
          "Name" : "Template Input Type" ,
          "Type" : "String",
        },
        {
          "Name" : "Final DNA Input for WGS (ug)" ,
          "Type" : "Int",
        },
        {
          "Name" : "DNA QC method" ,
          "Type" : "String",
        },
        {
          "Name" : "Desired Coverage (x)" ,
          "Type" : "int",
        },
      },
  }

  returnString, sendErr := json.Marshal( returnVal );
  if sendErr != nil{
    panic(sendErr);
  }

  file, err := os.Create("config2.txt")
    if err != nil {
        fmt.Println(err)
        return
    }

    count, err := file.Write( returnString )
    if err != nil {
        fmt.Println(err)
        file.Close()
        return
    }

    fmt.Println("Wrote %d bytes\n" , count )


}
