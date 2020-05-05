window.packages;
window.spreadsheet;
window.pacID;
window.token;
window.saved;
window.spreadsheetInitialized;

function initialize(callback){
  var params = getUrlVars();

  window.pacID = params["pacid"];
  window.token = params["id"];
  window.saved = true;
  window.spreadsheetInitialized = false;

  var param = {
    token : params["id"],
  };

  	$.ajax({
  		type: 'post',
  		url: '/initialize/',
  		dataType: 'json',
  		contentType: 'application/json',
  		data:JSON.stringify(param),

  		success: function(response){
            if( JSON.stringify(response) == "User not found")
            {
              alert( JSON.stringify(response) );
              return;
            }
              window.packages = response;

              callback();
  				}
  			});

}

function createButtons( divID, callback )
{
  var index;

  if( window.packages[0].packageID == undefined )
  {
    alert( "Something went wrong: Packages not initialized" );
    return;
  }

  for( index = 0; index < window.packages.length; index++ )
  {
    var img = document.createElement('img');
    var btn = document.createElement("input");
    btn.type = "button";
    btn.name = window.packages[index].packageID;
    btn.value = window.packages[index].packageDate;
    btn.id = window.packages[index].packageID;
    //btn.className = "Function";
    if( window.packages[index].packageID == window.pacID ) {
      btn.style.backgroundColor = "#737373";
    }
    else {
      btn.style.backgroundColor = "#00868B";
    }
    btn.style.color = "white";
    btn.style.width = "220px";
    btn.style.height = "45";
    btn.style.fontSize = "23px";
    btn.style.borderRadius = "20px";


    switch( window.packages[index].stepID )
    {
      case 1:
            img.src = '/tgen/images/metadata.png';
            img.title = "Filling out Metadata"
            break;
      case 2:
            img.src = '/tgen/images/shipped.png'
            img.title = "Shipped to TGen North"
            break;
      case 3:
            img.src = '/tgen/images/proc.png'
            img.title = "Received by TGen North, processing samples"
            break;
      case 4:
            img.src = '/tgen/images/234.png'
            img.title = "Results Ready!"
            break;
    }

    if( window.packages[index].errorCount > 0 ){
      img.src = '/tgen/images/errors.png';
      img.title = window.packages[index].errorCount + " Samples with Errors";
      img.setAttribute("onClick","navErrorPage(" + window.packages[index].packageID + ");");
    }

    img.height = 40;
    img.width = 40;

    btn.setAttribute("onClick","navSummaryPage(" + window.packages[index].packageID + ");");

    document.getElementById(divID).appendChild(btn);
    document.getElementById(divID).appendChild(img);
    document.getElementById(divID).appendChild(document.createElement("br"));
    document.getElementById(divID).appendChild(document.createElement("br"));
  }
  callback();
}

function getQRCodes( ){

  var param = {
    token: window.token,
    packageID : parseInt(window.pacID),
  };

  $.ajax({
    type: 'post',
    url: '/printQR/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function( response ){
      printJS({printable:response, type:'pdf', showModal:true });
    }
      });
}
function openMetadata( )
{
  var container = document.getElementById('table');
  var addColumnDiv = document.getElementById('addcol')
  var title = document.getElementById('metadataTitle');

  var param = {
    token: window.token,
    packageID : parseInt(window.pacID),
  };

  $.ajax({
    type: 'post',
    url: '/generateSpreadsheet/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){
      window.spreadsheet = new Handsontable(container, {
      data: response.metadata,
      startCols: response.columnHeaders.length,
      width: '75%',
      height: 700,
      rowHeights: 23,
      rowHeaders: true,
      colHeaders: response.columnHeaders,
      columns: response.spreadsheetConfig,
      licenseKey: 'non-commercial-and-evaluation',
      afterChange: function() {
        if( window.spreadsheetInitialized ) {
          window.saved = false;
        }
      },
      });

      for( index = 0; index < window.packages.length; index++ )
      {
        if( window.packages[index].packageID == window.pacID )
        {
          title.innerHTML = "Metadata for Package Created On " + window.packages[index].packageDate;

          if( window.packages[index].stepID < 2 )
          {
            document.getElementById('update').innerHTML += "<button type=\"button\" onclick=\"updatePackage();\">Save</button>";
            document.getElementById('insert').innerHTML += "<button type=\"button\" onclick=\"insertPackage();\">Submit</button>";
            document.getElementById('addsample').innerHTML += "<button type=\"button\" onclick=\"addNewSample();\" >Add Sample</button>";

            if( response.expandable ) {
              document.getElementById('addcol').innerHTML += "<button type=\"button\" onclick=\"addNewColumn();\">Add Column</button>";
            }
          }
          window.spreadsheetInitialized = true;
          return;
        }
      }
        }
      });
}
function getUrlVars() {
    var vars = {};
    var parts = window.location.href.replace(/[?&]+([^=&]+)=([^&]*)/gi, function(m,key,value) {
        vars[key] = value;
    });
    return vars;
}

function onSignIn(googleUser) {
  var id_token;
  // Useful data for client-side scripts:
  var profile = googleUser.getBasicProfile();
  googleUser.disconnect();
  // The ID token you need to pass to your backend:
  id_token = googleUser.getAuthResponse().id_token;
  console.log("ID Token: " + id_token);

  window.location.href = "tgen/index.html#?id=" + id_token;
}

function signOut() {
    var revokeAllScopes = function() {
      auth2.disconnect();
    }
    document.location.href = "/Sign_in.html";
}
function navCreatePackage() {
  window.location.href = "/tgen/CreatePackage.html#?id=" + window.token;
}

function navMetadata() {
  window.location.href = "/tgen/MetadataForm.html#?pacid=" + window.pacID + "&id=" + window.token;
}
function navErrorPage( packageID) {
  window.location.href = "/tgen/ErrorPage.html#?pacid=" + packageID + "&id=" + window.token;
}
function createNewPackage() {
  sampleNum = document.getElementById("samples").value;

  var param = {
    token: window.token,
    sampleNumber : parseInt(sampleNum),
  };

  $.ajax({
    type: 'post',
    url: '/newPackage/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){

      window.location.href = "/tgen/SummaryPage.html#?pacid=" + response + "&id=" + window.token;

    }
  })

}
function loadSummaryPage( ){
  title = document.getElementById("summaryTitle");
  var img = document.createElement("img");

  for( index = 0; index < window.packages.length; index++ )
  {
    if( window.packages[index].packageID == window.pacID )
    {
      img.src = 'images/step' + window.packages[index].stepID + '.png';
      document.getElementById("progressBar").appendChild(img);
      title.innerHTML = "Package Created On " + window.packages[index].packageDate;
      if( window.packages[index].errorCount > 0 ) {
        document.getElementById("summaryButtons").innerHTML += "<button class=\"botton_section\" onclick =\"navErrorPage(" + window.pacID + ");\">Resolve Errors</button><br>";
      }
      return;
    }
  }
}

function loadErrorPage(){
  var param = {
    token: window.token,
    packageID : parseInt(window.pacID),
  };

  $.ajax({
    type: 'post',
    url: '/checkErrors/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){
      var tab = document.createElement("TABLE");
      tab.setAttribute("id", "errorTable" );
      document.getElementById("errorDiv").appendChild(tab);

      var headerRow = document.createElement("TR");
      headerRow.setAttribute("id", "headerRow");
      document.getElementById("errorTable").appendChild(headerRow);

      var samp = document.createElement("TH");
      var text1 = document.createTextNode("Sample ID");
      samp.appendChild(text1);
      document.getElementById("headerRow").appendChild(samp);

      var err = document.createElement("TH");
      var text2 = document.createTextNode("Error Message");
      err.appendChild(text2);
      document.getElementById("headerRow").appendChild(err);

      var resolution = document.createElement("TH");
      var text3 = document.createTextNode("Error Resolution");
      resolution.appendChild(text3);
      document.getElementById("headerRow").appendChild(resolution);

      for( index = 0; index < response.length; index++ ) {
        var headerRow = document.createElement("TR");
        headerRow.setAttribute("id", "dataRow" + index );
        document.getElementById("errorTable").appendChild(headerRow);

        samp = document.createElement("TD");
        text1 = document.createTextNode(response[index].sampleID);
        samp.appendChild(text1);
        document.getElementById("dataRow" + index ).appendChild(samp);

        err = document.createElement("TD");
        text2 = document.createTextNode(response[index].errorMessage);
        err.appendChild(text2);
        document.getElementById("dataRow" + index ).appendChild(err);

        resolution = document.createElement("TD");
        text3 = document.createTextNode(response[index].errorResolution);
        resolution.appendChild(text3);
        document.getElementById("dataRow" + index).appendChild(resolution);
      }

    }
  })
}

function navSummaryPage( pacID ){
  window.location.href = "/tgen/SummaryPage.html#?pacid=" + pacID + "&id=" + window.token;
  if( window.location.href.includes("/tgen/SummaryPage.html") )
  {
    window.location.reload();
  }
}

function navErrorEmailPage(  ){
  {
    /*alert("Communicate your resolution to TGen point of contact:\n" +
    "\n\n To: TGen@gmail.com" +
    "\n\n From: Your Email" +
    "\n\n Subject: Sample Flag Resolution Submission #12345" +
    "\n\n Message: Eg: Please proceed with all Samples, excluding problematic samples" +
    "\n\t\t\t Please do what you can to recover data for the problematic samples, ect.");
    */
    window.location.href = "/tgen/SummaryPage.html#?pacid=" + window.pacID + "&id=" + window.token;;
  }
}


function updatePackage(){
  var data = [];
  //alert(window.spreadsheet.getColHeader(0));
  for( rowIndex = 0; rowIndex < window.spreadsheet.countRows(); rowIndex++ )
     {
       data[rowIndex] = {};
       for( colIndex = 0; colIndex < window.spreadsheet.countCols(); colIndex++ )
          {
           header = window.spreadsheet.getColHeader( colIndex );
           if( window.spreadsheet.getDataAtCell(rowIndex,colIndex) != null )
           {
             data[rowIndex][header] = window.spreadsheet.getDataAtCell(rowIndex,colIndex).toString();
           }
          }
     }

     var param = {
       token: window.token,
       packageID : parseInt(window.pacID),
       spreadsheet: data
     };

     $.ajax({
       type: 'post',
       url: '/updatePackage/',
       dataType: 'json',
       contentType: 'application/json',
       data:JSON.stringify(param),

       success: function(response){
         alert(response);
         window.saved = true;
       }
     })
}
function insertPackage(){
  var data = [];

  if( !confirm("Data cannot be edited after submission.\n Are you sure you want to submit?") )
  {
    return;
  }

  for( rowIndex = 0; rowIndex < window.spreadsheet.countRows(); rowIndex++ )
     {
       data[rowIndex] = {};
       for( colIndex = 0; colIndex < window.spreadsheet.countCols(); colIndex++ )
          {
           header = window.spreadsheet.getColHeader( colIndex );
           if( window.spreadsheet.getDataAtCell(rowIndex,colIndex) != null )
           {
             data[rowIndex][header] = window.spreadsheet.getDataAtCell(rowIndex,colIndex).toString();
           }
          }
     }

     var param = {
       token: window.token,
       packageID : parseInt(window.pacID),
       spreadsheet: data
     };

     //Save temp metadata first
     $.ajax({
       type: 'post',
       url: '/updatePackage/',
       dataType: 'json',
       contentType: 'application/json',
       data:JSON.stringify(param),

       success: function(response){
         window.saved = true;
       }
     })

     //Then insert into database
     $.ajax({
       type: 'post',
       url: '/insertPackage/',
       dataType: 'json',
       contentType: 'application/json',
       data:JSON.stringify(param),

       success: function(response){
           alert( response );
       }
     })
}

function addNewSample(){
  var param = {
    token: window.token,
    packageID : parseInt(window.pacID)
  };

  $.ajax({
    type: 'post',
    url: '/newSample/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){
      var newData = [];

      for( rowIndex = 0; rowIndex < window.spreadsheet.countRows(); rowIndex++ )
         {
           newData[rowIndex] = {};
           for( colIndex = 0; colIndex < window.spreadsheet.countCols(); colIndex++ )
              {
               header = window.spreadsheet.getColHeader( colIndex );
               if( window.spreadsheet.getDataAtCell(rowIndex,colIndex) != null )
               {
                 newData[rowIndex][header] = window.spreadsheet.getDataAtCell(rowIndex,colIndex).toString();
               }
              }
         }

      newData.push( JSON.parse(response) );
      window.spreadsheet.loadData( newData );
      window.spreadsheet.render();
    }
  })

}

function addNewColumn(){
  var columnHeader = prompt("Please Enter Your Column Header", "Column Header");

  var param = {
    token: window.token,
    packageID : parseInt(window.pacID),
  };

  $.ajax({
    type: 'post',
    url: '/generateSpreadsheet/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){
      response.columnHeaders.push(columnHeader);
      response.spreadsheetConfig.push( { data: columnHeader, } );

      alert( response.spreadsheetConfig);
      window.spreadsheet.updateSettings({
        columns: response.spreadsheetConfig,
        colHeaders: response.columnHeaders,
      });
    }
    })
}
function trackingNumber()
{
  var trackingNumber = prompt("Please Enter Your Tracking Number", "XXXX-XXXX-XXXX-XXXX");

  if( trackingNumber == null )
  {
    return;
  }

  var param = {
    token: window.token,
    packageID : parseInt(window.pacID),
    trackingNumber : trackingNumber
  };

  $.ajax({
    type: 'post',
    url: '/addTracking/',
    dataType: 'json',
    contentType: 'application/json',
    data:JSON.stringify(param),

    success: function(response){
      if( response == "Success" ) {
        alert("Thank You For Submitting Tracking Number: " + trackingNumber);
      } else {
        alert("Something went wrong");
      }

    }
  })

}
