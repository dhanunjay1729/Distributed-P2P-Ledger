/*
 File-based storage implementation for transactions.
 This implementation reads and writes transactions to a JSON file on disk.
 The FileStorage struct has a FilePath field that specifies the location of the 
 JSON file.
 The LoadTransactions method reads the file, unmarshals the JSON into a slice of
 Transaction structs, and returns it. If the file does not exist, it returns an empty slice.
 The SaveTransaction method loads existing transactions, appends the new transaction, 
 marshals the slice back to JSON, and writes it to the file.
 The TransactionExists method checks if a transaction with a given ID exists in the loaded transactions.
 
*/

package storage

import ("p2pledger/internal/models"
	    "io/ioutil"
	    "os"
	    "fmt" 
	    "encoding/json"
)

type FileStorage struct {
	FilePath string
}

func NewFileStorage(path string) *FileStorage {
	return &FileStorage{FilePath: path}
}

func (f *FileStorage) LoadTransactions() ([]models.Transaction, error) {
	// TODO: read from file


	

	jsonfile,err:=os.Open(f.FilePath)
	if err!=nil{
		 if os.IsNotExist(err) {
        return []models.Transaction{}, nil
    }
		fmt.Println("error opening the file ")
		fmt.Println(err.Error())
		return nil,err
	}
    defer jsonfile.Close()
    byteValue,_:=ioutil.ReadAll(jsonfile)
    var trans []models.Transaction
    err=json.Unmarshal(byteValue,&trans)
    if err!=nil{
    	fmt.Println("couldnt unmarshal the json file ")
    	fmt.Println(err.Error())
    	return nil,err
    }


    



	//ending returning a transaction array 
	return trans, nil
}

func (f *FileStorage) SaveTransaction(tx models.Transaction) error {
	// TODO:
	// 1. load existing
	// 2. append
	// 3. write back
    

    //note : i am saving a  single transaction 
    trans,err:=f.LoadTransactions()
    if err != nil {
    return err
}
    trans=append(trans,tx)
    byteValue,err:=json.Marshal(trans)
    if err!=nil{
    		fmt.Println("couldnt unmarshal the json file ")
    	fmt.Println(err.Error())
    	return err
    }
    err=ioutil.WriteFile(f.FilePath,byteValue,0644)
    if err!=nil{
    	fmt.Println("couldnt open the file ")
    	fmt.Println(err.Error())
    	return err 
    }





	return nil
}

func (f *FileStorage) TransactionExists(id string) (bool, error) {
	// TODO: check in loaded transactions



/*--------theres a problem here----------*/
/*--------should we store id speratly in a seprate slices instead of in file -------------*/
//O(n) method currently 


    trans, err := f.LoadTransactions()
    for _, t := range trans {
    if t.ID == id {
        return true, nil
    }
}
return false, err







}