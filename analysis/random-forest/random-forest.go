package randomforest

import (
	"fmt"
	"os"
	"readFile"
)

func main() {
	NUMCLASSES := 2
	NUMTREES := 10000
	RANDATTR := false
	if len(os.Args) < 3 {
		fmt.Println("Run ./RFThesis dataFile treeFile")
		os.Exit(-1)
	}

	inFile := os.Args[1]
	outBase := os.Args[2]

	//readFile.Read(inFile, uidPresent, classPresent)
	fmt.Println("Reading the data")
	//Modify this to query what data we want, need to label it!!!
	myData := readFile.Read(inFile, true, true)

	//DecisionForest.GenForest(allData, numClasses, numTrees, printRes, writeTrees, readTrees, randAttr, outBase)
	//Uncomment the following lines to test data
	//fmt.Println("Testing the forest")
	//DecisionForest.GenForest(myData, NUMCLASSES, NUMTREES, true, false, true, RANDATTR, outBase)
	//Uncomment the following lines to train the forest
	fmt.Println("Training the forest")
	DecisionForest.GenForest(myData, NUMCLASSES, NUMTREES, true, false, false, RANDATTR, outBase)
}
