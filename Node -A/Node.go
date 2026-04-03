package "Node"

import("fmt"
"net/http"
"github.com/gin-gonic/gin"
)

/*----------defining INTERFACES------------*/
type Nodefunc interface {
func LoadTransactions() []Transactions// this gets a gin context  i thinking am doing right 
func SaveTransactions() bool
func TrasactionExists() bool
func StartNode() bool 
func Load() bool
}


/*----------defining DATA STRUCTURE------------*/


/*-----------Trasaction----------------*/
type struct Transaction{

	ID        string `json:"id"`
    Data      string `json:"data"`
    Timestamp int64  `json:"timestamp"`
}


/*---------NODE-------------*/
type struct Node{
  PORT int64 
  Trans []Transactions
  offset int64 // purporse of the offset is to know uptowhat transaction u processsed ,most likely a redundant varibale 
  

}



/*---------Load call back for router--------*/
func (n *Node)Load(c *gin.Context){
     var temp []Transactions 

     if err:=ShouldBindJSON(&temp);err!=nil{
     	c.JSON(400,gin.H{"error":err.error()})
     }else{
     	fmt.Println("loading transactions")
     }
     append(n->Trans,temp)// saving the trasactions 


return true 
}



/*-------------ALL NODE methods-----------*/

func (n *Node)StartNode() bool{
	router:=gin.Default()

   /*--------PING TESTING---------*/
	router.GET("/ping", func(c *gin.Context){
		c.String(http.StatusOK,"pong")
	})

 
   /*---------ROUTE for loading trasactions-------*/
	router.GET("/load",Load)


}

func (n *Node)LoadTransactions() []Transactions {
// this needs to fetch transactions 
// i fucked up the logic 

}


func (n *Node)SaveTransactions() bool{
	//lol i need to fethc the source 
	
}





/*

func main() {
  router := gin.Default()

  router.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  http.ListenAndServe(":8080", router)
}
gin structure 
how do i make this for a node 
so when i intialaise a node using factory functin t need to start a server 
may be i will give a fied for port for the node and 
make fucntion to actiavte 


what other filed do i need ?





structure to agree upoon on the transction json 


*/









/*
goals 
3. Storage Layer
File: ledger.json

Functions:

LoadTransactions()
SaveTransaction(tx Transaction)
TransactionExists(id string)

/*