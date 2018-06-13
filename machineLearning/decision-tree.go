package ritaML

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
)

// Node -- basic node for our tree struct
// it will contain info on if this node is a Leaf
// what is the index of the value we should split on if this is not a Leaf
// The value we should use as our splitting point
// Finally this contains information on which class this will be if it's a leaf
type Node struct {
	Leaf       bool
	IndexSplit int
	SplitVal   float64
	Class      int
}

// Tree -- tree structure
// Details contains information at this particular level of the tree
// Used indicies keeps track of the indexes we've used for splitting
// Left is all nodes that go to the left
// Right is all nodes that go to the right
type Tree struct {
	Details      Node
	usedIndicies []int
	Left         *Tree
	Right        *Tree
}

// ClassAvg -- holds the averages of each class used for finding split
type ClassAvg struct {
	count    int
	averages []interface{}
	stdDev   []interface{}
}

//Train uses the dataset to train a tree for later predicition
func (decTree Tree) Train(trainSet []*Data, setVal, stopCond float64, classesCount int, removeRand bool) Tree {
	var setStack [][]*Data
	var treeStack []*Tree

	//Simplify the basic tree structure
	currTree := &decTree
	currSet := trainSet
	treeLen := 1

	featureLen := len(trainSet[0].FeatureSlice)
	if removeRand == true {
		currTree.usedIndicies = removeRandAttributes(featureLen)
	} else {
		currTree.usedIndicies = removePValAttributes(trainSet, featureLen)
	}

	//Ensure we have values before continuing, otherwise we get a runtime error
	for treeLen != 0 {
		var classes []ClassAvg
		var classSamples [][]*Data

		//Initialize all the class averages
		for i := 0; i < classesCount; i++ {
			var newClass ClassAvg

			classes = append(classes, newClass)
			classSamples = append(classSamples, *new([]*Data))

			classes[i].count = 0
		}

		//Average all the classes and find the split
		avgClass(currSet, classSamples, classes)
		left, right := currTree.findSplit(currSet, classes, setVal, stopCond, classesCount)

		//Check if we will continue or if we have a leaf node
		if currTree.Details.Leaf == false {
			//Copy the values to the right and the tree to a stack so we don't use
			//recursion add length to tree
			setStack = append(setStack, right)
			treeStack = append(treeStack, currTree.Right)
			currSet = left
			currTree = currTree.Left
			treeLen++
		} else {
			//get the length of the tree and set curr to the last element in the list
			treeLen--

			if treeLen > 0 {
				currTree, treeStack = treeStack[treeLen-1], treeStack[:treeLen-1]
				currSet, setStack = setStack[treeLen-1], setStack[:treeLen-1]
			}
		}
	}

	//Return the entire tree
	return decTree
}

//Test uses the dataset passed in to predict the dataset
func (decTree Tree) Test(allData []*Data) {
	misclassified := 0
	//Print the header so we can see results
	fmt.Printf("+-----------+----------+-------------------------+\n")
	fmt.Printf("| Predicted |  Actual  |           UID           |\n")
	fmt.Printf("+-----------+----------+-------------------------+\n")
	//For each datum in the data range run it through the completed tree
	for i, datum := range allData {
		prediction := decTree.GetClass(*datum, i)
		//Check if we have misclassified data, increasing misclassified count if we do
		if prediction != datum.Class {
			misclassified++
		}
		//Print that specific datum's classification result
		fmt.Printf("|     %d     |     %d    |", prediction, datum.Class)
		fmt.Printf("   %s   ", datum.UID)

		//This adds a little to the datum's list because it makes it easier to search
		//for anomalous traffic misclassified as normal traffic
		if prediction == 1 && datum.Class == 2 {
			fmt.Printf(" oops")
		}
		fmt.Printf("\n")
	}
	//Print footer and final tree results
	fmt.Printf("+-----------+----------+-------------------------+\n")

	fmt.Printf("%d out of %d wrongly classified\n", misclassified, len(allData))
	fmt.Printf("Misclassified: %f\n", float64(misclassified)/float64(len(allData)))
}

//GetClass returns an int value that refers to the class a value belongs to
func (decTree Tree) GetClass(datum Data, i int) int {
	currNode := decTree.GetTerminalNode(datum, i)

	if currNode != nil {
		return currNode.Details.Class
	}

	return 2
}

//GetTerminalNode iterates through a tree for a datum and then returns that node
//that datum is classified into
func (decTree Tree) GetTerminalNode(datum Data, i int) *Tree {
	currNode := &decTree

	for currNode.Details.Leaf == false {
		index := currNode.Details.IndexSplit
		if index < len(datum.FeatureSlice) {
			testVal := getVal(datum.FeatureSlice[index])
			if testVal <= currNode.Details.SplitVal {
				currNode = currNode.Left
			} else {
				currNode = currNode.Right
			}
		} else {
			return nil
		}
	}

	return currNode
}

//TODO move to write to a database collection
//WriteTree will save a tree to a file for use later on
func (decTree *Tree) WriteTree(filename string) {
	//Try to open the output file and return an error if one occurs
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error opening output file: ", filename)
		return
	}

	//Start the current node at the root of the tree and initialize the treeStack
	//for iteration
	currNode := decTree
	var treeStack []*Tree

	//Set length of tree equal to 1 (we have a root node) and start iterating through
	//the tree
	treeLen := 1
	for treeLen != 0 {
		file.WriteString(nodeToStr(currNode.Details))

		//As long as we don't have a leaf node we should go left and append the Right
		//node onto our tree stack so we can come back to it later
		if currNode.Details.Leaf == false {
			treeStack = append(treeStack, currNode.Right)
			currNode = currNode.Left
			treeLen++
		} else {
			//reduce the length of the tree and set curr to the last element in the list
			treeLen--

			if treeLen > 0 {
				currNode, treeStack = treeStack[treeLen-1], treeStack[:treeLen-1]
			}
		}
	}

	file.Close()
}

//TODO move to read from a database collection
//ReadTree will read a tree from the specified filename
func (decTree *Tree) ReadTree(filename string) error {
	//Try opening the input file and list any errors we may encounter
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("Error opening input file: ", filename)
		return err
	}

	//read the entire file into memory and split on new lines
	sDat := fmt.Sprintf("%s", file)
	datLines := strings.Split(sDat, "\n")

	//set the current node to the root and initialize values for iterating over
	//the trees
	currNode := decTree
	var treeStack []*Tree
	treeLen := 1
	lastNode := false

	//while we still have lines in a tree file we want to parse the line,
	//adding the data to our current node
	for _, line := range datLines {
		if !lastNode {
			currNode.Details.Leaf, currNode.Details.IndexSplit, currNode.Details.SplitVal, currNode.Details.Class, err = parseLine(line)
			if err != nil {
				return err
			}

			//While we aren't on a leaf node we want to initialize two child nodes
			//Move to the left and continue iterating
			if currNode.Details.Leaf == false {
				currNode.Left = new(Tree)
				currNode.Right = new(Tree)

				treeStack = append(treeStack, currNode.Right)
				currNode = currNode.Left
				treeLen++
			} else {
				//if we are at a leaf node, move to the most recent right child
				treeLen--
				if treeLen > 0 {
					currNode, treeStack = treeStack[treeLen-1], treeStack[:treeLen-1]
				} else {
					lastNode = true
				}
			}
		}
	}

	return nil
}

//NOTE: The following are private functions for the decision-tree code

//This function will remove random attributes to increase classification in a
//'non-deterministic' manner
func removeRandAttributes(attributeCount int) []int {
	var randAttributes []int

	removeAttributes := int(attributeCount/4 + 1)

	for i := 0; i < removeAttributes; i++ {
		index := rand.Int() % attributeCount

		randAttributes = append(randAttributes, index)
	}

	return randAttributes
}

//RemovePValAttributes will find & use the p-value to remove uninformative
//features to increase splits based on informative features
func removePValAttributes(dataSet []*Data, attributeCount int) []int {
	var pValAttributes []int
	var allPVals []float64

	//figure out how to do this...
	//first copy each attribute to it's own array
	for i := 0; i < attributeCount; i++ {
		normLikeAttributes := "["
		anomLikeAttributes := "["

		//add new piece of data to a string (string because it's easier to send to
		//a python command)
		for _, datum := range dataSet {
			if datum.Class == 1 {
				normLikeAttributes += strconv.FormatFloat(getVal(datum.FeatureSlice[i]), 'f', -1, 64) + ","
			} else {
				anomLikeAttributes += strconv.FormatFloat(getVal(datum.FeatureSlice[i]), 'f', -1, 64) + ","
			}
		}

		normLikeAttributes = normLikeAttributes[:len(normLikeAttributes)-1]
		anomLikeAttributes = anomLikeAttributes[:len(anomLikeAttributes)-1]

		normLikeAttributes += "]"
		anomLikeAttributes += "]"

		//here we calculate the p-value for a given attribute using python
		newPVal := getPVal(normLikeAttributes, anomLikeAttributes)
		allPVals = append(allPVals, newPVal)
	}

	//First we want to remove any p-values that return NaN
	for i, val := range allPVals {
		if val != val || val > 0.2 {
			pValAttributes = append(pValAttributes, i)
		}
	}

	// Also we will want to remove up to 8 attributes, removing the largest values
	// and working down
	// for len(pValAttributes) < 8 {
	// 	pValAttributes = append(pValAttributes, getNewRemoveVal(pValAttributes, allPVals))
	// }

	return pValAttributes
}

//getPVal gets a p-value for two range of values, it calls a basic python
//command since golang doesn't really have any support, then it returns a floating
//point version of the p-value
func getPVal(normAttributes, anomAttributes string) float64 {
	prog := "python"
	arg0 := "-c"
	arg1 := fmt.Sprintf("from scipy import stats; f, p = stats.f_oneway(%s, %s); print p", normAttributes, anomAttributes)

	cmd := exec.Command(prog, arg0, arg1)
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
		return 2.0
	}

	return float64frombytes(out)
}

//getNewRemoveVal will evaluate p-values and choose one to remove prefering large
//values
func getNewRemoveVal(removeIndexLst []int, pVals []float64) int {
	largestVal := pVals[0]
	largestIndex := 0

	for i, newVal := range pVals {
		if largestVal <= newVal && notInArray(removeIndexLst, i) {
			largestVal = newVal
			largestIndex = i
		}
	}
	return largestIndex
}

//notInArray will insure a potential new value isn't in the remove array
func notInArray(removeIndexLst []int, checkVal int) bool {
	for _, index := range removeIndexLst {
		if index == checkVal {
			return false
		}
	}

	return true
}

//float64frombytes will get a floating point number from a slice of bytes
func float64frombytes(bytes []byte) float64 {
	strVal := string(bytes)
	strVal = strVal[:len(strVal)-1]

	float, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		if strVal == "NaN" {
			return 1.0
		}
		return 0.0
	}

	return float
}

//parseLine will parse a single line from a tree file
func parseLine(line string) (bool, int, float64, int, error) {
	//Split the line on commas (basically we have a csv)
	lineItem := strings.Split(line, ",")
	if len(lineItem) < 4 {
		return false, 0, 0.0, 0, nil
	}

	//the file structure is bool, int, float, int
	//which corresponds to leaf node, split attribute index, split value, and node class
	leafNode, err := strconv.ParseBool(lineItem[0])
	if err != nil {
		return false, 0, 0.0, 0, err
	}
	splitIndex, err := getRegInt(lineItem[1])
	if err != nil {
		return false, 0, 0.0, 0, err
	}
	splitValue, err := strconv.ParseFloat(lineItem[2], 64)
	if err != nil {
		return false, 0, 0.0, 0, err
	}
	class, err := getRegInt(lineItem[3])
	if err != nil {
		return false, 0, 0.0, 0, err
	}

	return leafNode, splitIndex, splitValue, class, nil
}

//This function will get a base 32 integer from a string value
func getRegInt(line string) (int, error) {
	var retVal int

	i64, err := strconv.ParseInt(line, 10, 32)
	if err != nil {
		return retVal, err
	}

	retVal = int(i64)

	return retVal, nil
}

//nodeToStr will take a node value and return a csv line representation
//of that node for forest storage
func nodeToStr(currNode Node) string {
	leafStr := strconv.FormatBool(currNode.Leaf)
	indexSplit := strconv.Itoa(currNode.IndexSplit)
	splitVal := strconv.FormatFloat(currNode.SplitVal, 'f', 24, 64)
	classStr := strconv.Itoa(currNode.Class)

	return leafStr + "," + indexSplit + "," + splitVal + "," + classStr + "\n"
}

//TODO consider shortening this function!!!
//findSplit will take all the attributes and find the best split value
//however this requires finding and comparing all possible split values
func (decTree *Tree) findSplit(currData []*Data, classes []ClassAvg, setVal, stopCond float64, numClasses int) ([]*Data, []*Data) {
	if stoppingCond(currData, stopCond, numClasses) {
		decTree.Details.Leaf = true
		decTree.Details.Class = getMajority(currData, numClasses)
		return nil, nil
	}

	numFields := len(currData[0].FeatureSlice)

	var splitVals []float64
	var entropys []float64
	var left []*Data
	var right []*Data

	//for each attribute
	//handle the calculation of the entropy for that attribute, needed to find
	//split
	for i := 0; i < numFields; i++ {
		indexUsed := false
		//for each used index initialize the entropy to a huge value and the split to a small value
		for _, temp := range decTree.usedIndicies {
			if temp == i {
				entropys = append(entropys, setVal)
				splitVals = append(splitVals, 0)
				indexUsed = true
				break
			}
		}

		//ensure we haven't used this index before calculating the entropy
		if indexUsed == false {
			var tempVals []float64
			var averages []float64
			var stdDevs []float64
			var tempEntropys []float64

			//For each class in the classes slice
			for _, class := range classes {
				//if a class is empty we should initialize it
				if len(class.averages) == 0 {
					averages = append(averages, setVal)
					stdDevs = append(stdDevs, setVal)
					tempVals = append(tempVals, setVal)
					tempEntropys = append(tempEntropys, setVal)
				} else {
					//if we have something that is initialized we can append the new values
					//the average attribute value for that class, the standard deviation
					//a proposed split value and the entropy of using that split value
					//Try to change the averages and stdDev to min and max for each index
					//generate a set of ~5 numbers in that range to try
					averages = append(averages, GetFloatReflectVal(class.averages[i]))
					stdDevs = append(stdDevs, GetFloatReflectVal(class.stdDev[i]))
					//TODO try not adding the STDDev and then try subtracting the STDDev see if that improves classification
					tempVals = append(tempVals, averages[len(averages)-1]+stdDevs[len(stdDevs)-1])
					tempEntropys = append(tempEntropys, findEntropy(i, len(classes), averages[len(averages)-1], stdDevs[len(stdDevs)-1], currData))
				}
			}

			//Find which class has the better split value
			tempIndex, tempEntropy := findLeast(tempEntropys)
			//_, tempEntropy = findLeast(tempEntropys)

			//add that entropy and split value to our list for later use
			splitVals = append(splitVals, tempVals[tempIndex])
			entropys = append(entropys, tempEntropy)
		}
	}

	//Here we want to find the smallest entropy to use in the
	index := findIndex(entropys)

	//Initialize the node values
	decTree.Details.Leaf = false
	decTree.Details.SplitVal = splitVals[index]
	decTree.Details.IndexSplit = index

	//create new children nodes
	decTree.Left = new(Tree)
	decTree.Right = new(Tree)

	//Add the index to the used indicies list if a binary value
	if decTree.Details.IndexSplit > 7 && decTree.Details.IndexSplit < 27 {
		decTree.Left.usedIndicies = append(decTree.usedIndicies, decTree.Details.IndexSplit)
		decTree.Right.usedIndicies = append(decTree.usedIndicies, decTree.Details.IndexSplit)
	}

	for _, elem := range currData {
		compVal := getVal(elem.FeatureSlice[index])

		if compVal <= splitVals[index] {
			left = append(left, elem)
		} else {
			right = append(right, elem)
		}
	}

	//Decided if we have a good split, if all values go left or right we should end
	if len(left) == len(currData) {
		decTree.Details.Leaf = true
		decTree.Details.Class = getMajority(currData, numClasses)
		left, right = nil, nil
	} else if len(right) == len(currData) {
		decTree.Details.Leaf = true
		decTree.Details.Class = getMajority(currData, numClasses)
		left, right = nil, nil
	}

	return left, right
}

//getVal will get a value from an abstract interface type
func getVal(val interface{}) float64 {
	//I think I'm not handling strings correctly, If I get a string I'm assinging
	//it to a 0.0 value then can split on it...oops
	testVal := 0.0
	//switch on the detected type of variable, currently we support
	//float64 and bool values
	switch val.(type) {
	case float64:
		testVal = GetFloatReflectVal(val)
	case bool:
		testVal = GetBoolReflectVal(val)
	}

	return testVal
}

//avgClass will average all attributes for all classes and return the running average
//and standard deviation
//Here try to generate 5 random values for all the data in the min and max data range
func avgClass(allData []*Data, classSamples [][]*Data, classes []ClassAvg) {
	/*for each piece of data adjust the class by one and find the running average for that
	classes attributes*/
	for _, datum := range allData {
		classIndex := datum.Class - 1

		//set the averages to the running average, increase class count and append the datum
		//to the appropriate class sample array
		classes[classIndex].averages = runningAvg(classes[classIndex].averages, *datum, classes[classIndex].count)
		classes[classIndex].count++
		classSamples[classIndex] = append(classSamples[classIndex], datum)
	}
	//var mins []interface{}
	//var maxs []interface{}
	//for _, datum := range allData {
	//
	//}

	/*Find the standard deviation after all averages are found
	here just append zero*/
	// for i, class := range classes {
	for i := range classes {
		// classes[i].stdDev = findStds(classSamples[i], class)
		classes[i].stdDev = noDev(classSamples[i])
	}
}

func noDev(classSam []*Data) []interface{} {
	var stdDev []interface{}

	//if we don't have any instances just return an empty interface
	if len(classSam) == 0 {
		return stdDev
	}

	//get count of attributes
	featureLen := len(classSam[0].FeatureSlice)

	//for every attribute in a list of data points find the standard deviation in the usual way
	for i := 0; i < featureLen; i++ {
		stdDev = append(stdDev, 0.0)
	}

	return stdDev
}

//getMajority will find the majority class for whatever data is passed into it (limited
// by the number of classes we have)
func getMajority(data []*Data, numClasses int) int {
	counts := make([]int, numClasses)

	//count each occurance of a class
	for _, datum := range data {
		counts[datum.Class-1]++
	}

	//set max class index to zero and compare all the class count
	max := 0
	for i := 1; i < numClasses; i++ {
		if counts[i] > counts[max] {
			max = i
		}
	}

	//return class majority (remember to add one for off by one error)
	return max + 1
}

//stoppingCond will check if we have reached the desired purity of a set
func stoppingCond(nodeData []*Data, stopCond float64, classes int) bool {
	count := make([]int, classes)
	percent := make([]float64, classes)

	//Count each occurance of a class so we can check the purity
	for _, elem := range nodeData {
		count[elem.Class-1]++
	}

	//for each class count check the percentage of each class count to see if we have
	//reached the threshold
	for i := range count {
		percent[i] = float64(count[i]) / float64(len(nodeData))
		if percent[i] >= stopCond {
			return true
		}
	}

	return false
}

//findEntropy will check the entropy given an average and standard deviation as a split
// value and
func findEntropy(valueIndex, classCount int, avg, stdDev float64, nodeData []*Data) float64 {
	var classInstances []float64
	var classEntropies []float64
	var classWeights []float64

	//initialze class count, entropy and weights to zero
	for i := 0; i < classCount; i++ {
		classInstances = append(classInstances, 0.0)
		classEntropies = append(classEntropies, 0.0)
		classWeights = append(classWeights, 0.0)
	}

	//for each datum in our dataset we should get the attribute we are splitting on, and
	//that datums class, add a 1 or 0 based on how the split value affects that datum
	for _, datum := range nodeData {
		instance := getVal(datum.FeatureSlice[valueIndex])
		classIndex := datum.Class - 1

		//TODO try not adding the STDDev and then try subtracting the STDDev see if that improves classification
		classInstances[classIndex] += countClass(instance, avg+stdDev)
	}

	//find the length of all data to set the class weights
	lenData := float64(len(nodeData))

	entropy := 0.0
	for i := 0; i < classCount; i++ {
		//Assuming we have at least one class instance we can find purity of that split
		if classInstances[i] > 0 {
			classWeights[i] = classInstances[i] / lenData
			classEntropies[i] = classWeights[i] * math.Log2(classWeights[i])
			entropy += classWeights[i] * classEntropies[i]
		}
	}

	//TODO also try to remove the *-1 just to see...
	return entropy * -1
}

//countClass will check which direction the split value will affect a value,
//returns 1 if we go left, and 0 if we go right
func countClass(instance float64, splitVal float64) float64 {
	if instance <= splitVal {
		return 1
	}

	return 0
}

//initializeAvgs will initialize an average value given the type of an attribute for all
//attributes we have
func initializeAvgs(example Data) []interface{} {
	var newAvgVals []interface{}

	//switch on the variable type of an attribute
	for i := range example.FeatureSlice {
		switch example.FeatureSlice[i].(type) {
		//for both floats and bools set initial value to 0 (false if it's bool)
		case float64:
			newAvgVals = append(newAvgVals, 0.0)
		case bool:
			newAvgVals = append(newAvgVals, 0.0)
		//for a string initialize the initial value to an empty string, I still need to
		//figure out a good way to do this
		case string:
			newAvgVals = append(newAvgVals, "")
		}
	}

	return newAvgVals
}

//findLeast will find the smallest value in a floating point value array
func findLeast(values []float64) (int, float64) {
	leastIndex := 0
	leastVal := values[0]

	//for each value in the value array check if it's less than the current minimum,
	//if it is reset current minimum and current minimum index
	for i, val := range values {
		if val < leastVal {
			leastVal = val
			leastIndex = i
		}
	}

	return leastIndex, leastVal
}

//runningAvg will calculate the running average of a generic interface, given a new
//piece of datum, and count of pervious data points
func runningAvg(oldAvgs []interface{}, newVal Data, n int) []interface{} {
	//if we have an empty inital average we need to initialize the average values
	if len(oldAvgs) < len(newVal.FeatureSlice) {
		oldAvgs = initializeAvgs(newVal)
	}

	//for every attribute calculate an approximate running sum value, add the new value
	//then divide the approximate running sum value by one greater than count of
	//contributing data points
	for i := range newVal.FeatureSlice {
		//keep track of the running averages
		temp := getVal(oldAvgs[i]) * float64(n)
		temp += getVal(newVal.FeatureSlice[i])
		oldAvgs[i] = temp / float64(n+1)
	}

	//return the new running average
	return oldAvgs
}

//findStds will find the standard deviation given a set of data points and a list
//of attribute averages for those data points
func findStds(classSam []*Data, class ClassAvg) []interface{} {
	var stdDev []interface{}

	//if we don't have any instances just return an empty interface
	if len(classSam) == 0 {
		return stdDev
	}

	//get count of attributes
	featureLen := len(classSam[0].FeatureSlice)

	//for every attribute in a list of data points find the standard deviation in the usual way
	for i := 0; i < featureLen; i++ {
		classTotal := 0.0
		for _, sample := range classSam {
			class.stdDev = append(class.stdDev, 0.0)
			//reflect the type of the feature slice index handle float, bool and string (don't worry about bool and str yet)
			sampleVal := getVal(sample.FeatureSlice[i])
			classVal := getVal(class.averages[i])
			classTotal += math.Pow((sampleVal - classVal), 2)
		}

		//add the standard deviation to the standard deviation list
		stdDev = append(stdDev, classTotal/float64(class.count))
	}

	return stdDev
}

//findIndex will return the attribute index to be used for our split value
func findIndex(entropyVals []float64) int {
	minVal := entropyVals[0]
	minIndex := 0

	//for every split value (same as the entropy value) find the smallest entropy, highest
	// purity of split values
	for i, contender := range entropyVals {
		if contender < minVal {
			minIndex = i
			minVal = contender
		}
	}

	return minIndex
}

//GetFloatReflectVal takes an interface value and returns it as a float64 type
func GetFloatReflectVal(val interface{}) float64 {
	v := reflect.ValueOf(val)
	v = reflect.Indirect(v)

	floatVal := v.Convert(reflect.TypeOf(0.0))
	return floatVal.Float()
}

//GetBoolReflectVal takes an interface value and returns it as a bool type
func GetBoolReflectVal(val interface{}) float64 {
	v := reflect.ValueOf(val)
	v = reflect.Indirect(v)

	boolVal := v.Convert(reflect.TypeOf(true))

	if boolVal.Bool() == true {
		return 1.0
	}
	return 0.0
}

//GetStrReflectVal takes an interface value and returns it as a string value
func GetStrReflectVal(val interface{}) string {
	v := reflect.ValueOf(val)
	v = reflect.Indirect(v)

	strVal := v.Convert(reflect.TypeOf(""))
	return strVal.String()
}
