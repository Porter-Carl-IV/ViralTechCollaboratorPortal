package main

import (
    "fmt"
    "encoding/json"
    "os"
)

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

func main(){
  var returnVal = Config {
      Expandable: true,
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
