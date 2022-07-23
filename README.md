#  servergoQbites
[![Go](https://img.shields.io/badge/--00ADD8?logo=go&logoColor=ffffff)](https://golang.org/)
[![BitCoin](https://badgen.net/badge/icon/bitcoin?icon=bitcoin&label)](https://bitcoin.org)

	

<img src = "https://github.githubassets.com/images/mona-loading-dark.gif" height=20/> servergoQbites is graphql server for ai indicator project

## :hammer_and_wrench: Installation 
run your mongodb database on port 27017
install [go](https://go.dev/doc/install) then run 

```bash
go run server.go
```

## :building_construction: Usage
### get list of all crypto coin in the database
```javascript
query{
  Allcrypto
}
```
### get data of specific crypto 
```javascript
query{
    crypto(name: $name){
      name
      time
    	projection
    	gainlose{
        M
        W
        D
        
      }
    	
          data{
      hcl{
        opentime
        Open
        High
        Low
        Close
          Volume
        
        
      }
        formula{
          rsi
          rsiK
          rsiD
          aroonu
          aroond
          macd
          histogram
         
        }
        ai{
          bigmome{
            BUY2
            BUY1
            ambi
            
          }
          sell{
            amb0
            amb1
            amb2
            amb3
            amb99
            
          }
          smallmome{
            amo
            ci
            
          }
          buy{
            ambb
            ambb5
            ww6
            ww7
            
          }
          mome{
            amb14
            amb15
            amb13
            amb55
            
          }
          other{
            amo
            amo1
            BUYSELL
            BUYSELLevel
            
          }
          
        }
      }
    }
  }
```
### get user data
```javascript
query{
  User(nameuser: $nameuser) {
      nameuser
      balance {
        cryptoName
        totale
      }
    }
}

```



## :pirate_flag: License
[MIT](https://choosealicense.com/licenses/mit/)
