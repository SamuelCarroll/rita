package ritaML

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelCarroll/DecisionTree"
)

func boolFloatToStr(boolFloat float64) string {
	if boolFloat == 0.0 {
		return "False"
	}
	return "True"
}

func getString(datum Data, prediction int) []string {
	var datumStr []string

	datumStr = append(datumStr, strconv.Itoa(prediction))
	datumStr = append(datumStr, datum.UID)

	for _, feature := range datum.FeatureSlice {
		var appStr string
		switch feature.(type) {
		case float64:
			datum := DecisionTree.GetFloatReflectVal(feature)
			appStr = strconv.FormatFloat(datum, 'f', 24, 64)
		case bool:
			datumBool := DecisionTree.GetBoolReflectVal(feature)
			appStr = boolFloatToStr(datumBool)
		case string:
			appStr = DecisionTree.GetStrReflectVal(feature)
		}
		datumStr = append(datumStr, appStr)
	}

	return datumStr
}

func labelData(forest []Tree, newData []*Data) []*Data {
	for i, elem := range newData {
		var guesses []int
		for _, tree := range forest {
			estimatedClass := tree.GetClass(*elem, i)

			guesses = append(guesses, estimatedClass)
		}

		newData[i].Class = getMajorityGuess(guesses)
	}

	return newData
}

//SemiSupervisedLearning will implement a SSL method
func SemiSupervisedLearning(labeledData, unlabeledData []*Data, numClasses, numTrees, generation int, printRes, writeTrees, readTrees bool, outBase string) ([]Tree, []*Data) {
	var supervisedForest []Tree
	//If this is the first generation we should create the inital forest
	//If it's not the first generation we should read in the forest
	if generation == 0 {
		supervisedForest, _ = GenForest(labeledData, numClasses, numTrees, printRes, writeTrees, readTrees, true, outBase)
	} else {
		supervisedForest = testRead(labeledData, false, outBase, generation*numTrees)
	}

	newData := labelData(supervisedForest, unlabeledData)

	for _, newDatum := range newData {
		labeledData = append(labeledData, newDatum)
	}

	finalForest, finalData := GenForest(labeledData, numClasses, numTrees, printRes, writeTrees, readTrees, true, outBase)

	return finalForest, finalData
}

//GenForest builds a decision tree of a specified size with a given number of classes and returns the forest and the data used to test the tree
func GenForest(allData []*Data, numClasses, numTrees int, printRes, writeTrees, readTrees, randAttr bool, outBase string) ([]Tree, []*Data) {
	var decTree Tree
	var decForest []Tree
	setVal := 100000000000.0 //big value to ignore a already used split feature value
	stopCond := 0.85         //point were we stop training if we have this percent of a single class
	rand.Seed(time.Now().UTC().UnixNano())

	//here we want to run association rules... (find a way to save what we have previously accomplished)

	//if we want to specify to read previously made trees just test that forest
	//with all that data
	if readTrees == true {
		allData = readAssociations(allData, outBase)
		testRead(allData, printRes, outBase, numTrees)
		return nil, nil
	}

	allData = findAssociations(allData, outBase)

	//TODO see if we get speed boost by modifying bagging to return a single
	//training set, put it in loop
	//call bagging, get back a slice of training data and a slice of testing data
	trainSets, testSets := bagging(allData, numTrees)

	tenPercent := len(trainSets) / 10

	//get the start time of training/building the forest so we know how long it takes
	start := time.Now()
	//For each bagging training set we generated make a tree
	for i, trainData := range trainSets {
		//here we use the last bool to determine random or p-value variable reduction
		decTree = decTree.Train(trainData, setVal, stopCond, numClasses, randAttr)
		decForest = append(decForest, decTree)

		if i%tenPercent == 0 {
			fmt.Println(10*i/tenPercent, "% done")
		}
	}
	elapsed := time.Since(start)
	fmt.Println("It took ", elapsed, " to train the forest")

	//Start testing on the OOB data
	misclassified := 0
	if printRes == true {
		//fmt.Printf("+-----------+----------+-------------------------+\n")
		//fmt.Printf("| Predicted |  Actual  |           UID   \t |\n")
		//fmt.Printf("+-----------+----------+-------------------------+\n")
		start = time.Now()
	}

	//For every element in the OOB set run it through every tree in the forest
	//To classify that given element
	var allPredictions []int
	for i, elem := range testSets {
		var guesses []int
		for _, tree := range decForest {
			estimatedClass := tree.GetClass(*elem, i)

			guesses = append(guesses, estimatedClass)
		}

		prediction := getMajorityGuess(guesses)
		if prediction != elem.Class {
			misclassified++
		}
		//if printRes {
		//fmt.Printf("|     %d     |     %d    |   %s\t |", prediction, elem.Class, elem.UID)
		//if prediction == 1 && elem.Class == 2 {
		//	fmt.Printf("\t oops")
		//}
		//fmt.Printf("\n")
		//}
		if writeTrees {
			allPredictions = append(allPredictions, prediction)
		}
	}
	//Print the end of the data if we specify print
	if printRes {
		elapsed = time.Since(start)
		//fmt.Printf("+-----------+----------+-------------------------+\n")

		fmt.Printf("%d out of %d wrongly classified\n", misclassified, len(testSets) /*len(testData)*/)
		fmt.Printf("Misclassified: %f%%\n", (float64(misclassified) / float64(len(testSets)) * 100.0))

		fmt.Println("It took ", elapsed, " to test the forest")
	}

	//If we want to write the trees to the current directory
	if writeTrees {
		for i, tree := range decForest {
			tree.WriteTree(outBase + strconv.Itoa(i) + ".txt")
		}
	}

	return decForest, testSets
}

//This will test a forest that is stored in a series of files
func testRead(dataSet []*Data, printRes bool, outBase string, numTrees int) []Tree {
	var decForest []Tree
	misclassified := 0
	//Positive is anomalous, negative is normal
	truePositive := 0
	trueNegative := 0
	falsePositive := 0
	falseNegative := 0

	//Read in all trees and add each tree to the forest
	for i := 0; i < numTrees; i++ {
		var tempTree Tree
		err := tempTree.ReadTree(outBase + strconv.Itoa(i) + ".txt")
		if err != nil {
			fmt.Println(err)
			return nil
		}
		decForest = append(decForest, tempTree)
	}

	fmt.Println("Forest Read")

	start := time.Now()

	//If this is set to true we assume we want to build this all over again
	if printRes == true {
		//For each datum run it through the forest, writing the results
		tenPercent := 10
		if len(dataSet) >= 10 {
			tenPercent = len(dataSet) / 10
		}
		//datLen := len(dataSet)
		for i, elem := range dataSet {
			var guesses []int

			if i%tenPercent == 0 {
				fmt.Println(10*i/tenPercent, "% done")
			}

			for _, tree := range decForest {
				estimatedClass := tree.GetClass(*elem, i)

				guesses = append(guesses, estimatedClass)
			}

			prediction := getMajorityGuess(guesses)
			if prediction == 1 && elem.Class == 1 {
				trueNegative++
			} else if prediction == 2 && elem.Class == 2 {
				truePositive++
			} else if prediction == 1 && elem.Class == 2 {
				falseNegative++
			} else if prediction == 2 && elem.Class == 1 {
				falsePositive++
			}

			if prediction != elem.Class {
				misclassified++
			}
		}
		elapsed := time.Since(start)

		fmt.Printf("%d out of %d wrongly classified\n", misclassified, len(dataSet))
		fmt.Printf("Misclassified: %f%%\n", (float64(misclassified)/float64(len(dataSet)))*100.0)
		//Positive is anomalous, negative is normal
		fmt.Printf("\tAnom Correctly Labeled: %d\n\tNorm Correctly Labeled: %d\n", truePositive, trueNegative)
		fmt.Printf("\tAnom incorrectly Labeled: %d\n\tNorm incorrectly Labeled: %d\n", falseNegative, falsePositive)
		fmt.Println("It took", elapsed, "to test", len(dataSet), "elements")
	}

	return decForest
}

//bagging will randomly generate a series of different data to train the forest
//and to test the forest when it is trained
func bagging(allData []*Data, numTrees int) ([][]*Data, []*Data) {
	dataLen := len(allData)
	dataUsed := make([]bool, len(allData))

	var trainSets [][]*Data
	var testSets []*Data

	//Generate a number of sets to train different trees on that data
	for i := 0; i < numTrees; i++ {
		//randomly select an index from 0-dataLen add that element to the end of a tempset
		//at the end of that
		var newTestSet []*Data
		var newTrainSet []*Data
		var usedIndices []int

		//While we don't have all the training sets make generate a random index and
		//add it to used indicies list add the corresponding datum to a training set
		//NOTE: we want to do this with replacement, each datum can be selected
		//multiple times
		for len(newTrainSet) < dataLen {
			randIndex := rand.Intn(dataLen)
			usedIndices = append(usedIndices, randIndex)
			newTrainSet = append(newTrainSet, allData[randIndex])
		}

		//generate the test dataset, add trainset to training sets, add to testset
		newTestSet, dataUsed = getNewTestSet(allData, usedIndices, dataLen, dataUsed)
		trainSets = append(trainSets, newTrainSet)
		testSets = append(testSets, newTestSet...)
	}

	return trainSets, testSets
}

//getNewTestSet will use all the data read from a file, the used indicies to
//generate the test set for use after we generate the forest
func getNewTestSet(allData []*Data, usedIndices []int, dataLen int, dataUsed []bool) ([]*Data, []bool) {
	//initialize the newSet to be empty, we will fill it up and return it
	var newSet []*Data
	//for each datum in the data we will check if it's index is used, appending it
	//to our test set if it hasn't been used
	for j := 0; j < dataLen; j++ {
		indexUsed := false
		for _, usedIndex := range usedIndices {
			if usedIndex == j {
				indexUsed = true
				dataUsed[usedIndex] = true
				break
			}
		}
		if !indexUsed && dataUsed[j] == false {
			newSet = append(newSet, allData[j])
		}
	}
	return newSet, dataUsed
}

//TODO change this from a simple majority vote to a Baysian network analysis of the
//accuracy of the tree for the training data
//getMajorityGuess will get the results from every tree and guess which class
//the datum belongs to by a majority vote
func getMajorityGuess(guesses []int) int {
	var classGuesses []int

	//tally each guess into the proper class guess
	for _, guess := range guesses {
		if (guess) > len(classGuesses) {
			for (guess) > len(classGuesses) {
				classGuesses = append(classGuesses, 0)
			}
		}
		classGuesses[guess-1]++
	}

	maxIndex := 1

	//find the largest value, compensate for off by one error and return
	for i, contender := range classGuesses {
		if contender > classGuesses[maxIndex-1] {
			maxIndex = i + 1
		}
	}

	return maxIndex
}

//modify this to only find the top 30 associationRules
func findAssociations(allData []*Data, outBase string) []*Data {
	//look through booleanValues to see which are good to combine
	var initialSet []int
	for i := 8; i < 27; i++ {
		initialSet = append(initialSet, i)
	}
	associationRules := powerSet(allData, initialSet)

	err := writeAssociations(associationRules, outBase)
	if err != nil {
		return nil
	}

	for i, elem := range allData {
		for _, rule := range associationRules {
			appVal := 0.0
			for _, index := range rule {
				appVal += DecisionTree.GetFloatReflectVal(elem.FeatureSlice[index])
			}
			allData[i].FeatureSlice = append(allData[i].FeatureSlice, appVal)
		}
	}

	return allData
}

func truePowerSet(initialSet []int) [][]int {
	var allSets [][]int

	for _, elem := range initialSet {
		setsLen := len(allSets)
		var newSet []int
		newSet = append(newSet, elem)

		for i := 0; i < setsLen; i++ {
			allSets = append(allSets, deepAppend(allSets[i], elem))
		}
		allSets = append(allSets, newSet)
	}

	return allSets
}

//Check if the set is good, needed to generate the power set to save processing
//time
func goodSet(allData []*Data, checkSet []int) (bool, float64) {
	truePresent := 0.0
	falsePresent := 0.0
	class1Count := 0.0
	class2Count := 0.0

	//For each datum in the dataset we want to check if there is a high correlation
	//for each class
	for _, elem := range allData {
		val := 0.0
		for _, index := range checkSet {
			val += DecisionTree.GetFloatReflectVal(elem.FeatureSlice[index])
		}

		val = val / float64(len(checkSet))
		//we want to go a little bit smaller than 1, just in case of a rounding error
		if elem.Class == 1 {
			class1Count++
			if val > 0.9 {
				truePresent++
			}
		} else if elem.Class == 2 {
			class2Count++
			if val > 0.9 {
				falsePresent++
			}
		}
	}

	//https://en.wikipedia.org/wiki/Lift_(data_mining)
	truePConf := float64(truePresent) / float64(class1Count)
	falsePConf := float64(falsePresent) / float64(class2Count)
	class1Sprt := float64(class1Count) / float64(len(allData))
	class2Sprt := float64(class2Count) / float64(len(allData))

	lift1 := truePConf / class1Sprt
	lift2 := falsePConf / class2Sprt
	if lift1 > 1.0 {
		return true, lift1
	} else if lift2 > 1.0 {
		return true, lift2
	}

	return false, 0.0
}

//generate a power set of high correlation data
func powerSet(allData []*Data, initialSet []int) [][]int {
	var allSets [][]int
	var singleSets [][]int
	var allLifts []float64

	for _, elem := range initialSet {

		setsLen := len(allSets)
		var newSet []int
		//check if we have a high rate in appearance before continuing
		newSet = append(newSet, elem)

		continueSingle, _ := goodSet(allData, newSet)
		if continueSingle {
			for i := 0; i < setsLen; i++ {
				//Here evaluate the dataset to see if we have a high correlation
				//if we do, add to the set
				goodBool, setLift := goodSet(allData, deepAppend(allSets[i], elem))
				if goodBool {
					allSets = append(allSets, deepAppend(allSets[i], elem))
					allLifts = append(allLifts, setLift)
				}
			}
			singleLen := len(singleSets)
			for i := 0; i < singleLen; i++ {
				goodBool, setLift := goodSet(allData, deepAppend(singleSets[i], elem))
				if goodBool {
					allSets = append(allSets, deepAppend(singleSets[i], elem))
					allLifts = append(allLifts, setLift)
				}
			}
			if len(newSet) == 1 {
				singleSets = append(singleSets, newSet)
			}
		}
	}

	if len(allSets) < 30 {
		return allSets
	}

	topSets := getBestSets(allSets, allLifts)
	return topSets
}

func getBestSets(allSets [][]int, allLifts []float64) [][]int {
	var bestSets [][]int
	var bestLiftVals []float64
	var bestLifts []int

	for i := 0; i < 30; i++ {
		bestLiftVals = append(bestLiftVals, 0.0)
		bestLifts = append(bestLifts, 0)
	}

	for newIndex, i := range allLifts {
		lowestVal := 0
		for j, k := range bestLiftVals {
			if k < bestLiftVals[lowestVal] {
				lowestVal = j
			}
		}
		if i < bestLiftVals[lowestVal] {
			bestLiftVals[lowestVal] = i
			bestLifts[lowestVal] = newIndex
		}
	}

	for _, i := range bestLifts {
		bestSets = append(bestSets, allSets[i])
	}

	return bestSets
}

func deepAppend(set []int, val int) []int {
	newSet := make([]int, len(set)+1)

	for i := range set {
		newSet[i] = set[i]
	}

	newSet[len(set)] = val

	return newSet
}

func isBool(feature interface{}) bool {
	switch feature.(type) {
	case bool:
		return true
	}

	return false
}

func writeAssociations(combinations [][]int, outBase string) error {
	outputName := outBase + "associationRules.txt"

	file, err := os.Create(outputName)
	if err != nil {
		return err
	}

	for _, combo := range combinations {
		var comboStr string

		for _, elem := range combo {
			comboStr += strconv.Itoa(elem) + ","
		}
		comboStr = comboStr[:len(comboStr)-1]
		comboStr += "\n"

		file.WriteString(comboStr)
	}

	return nil
}

func readAssociations(allData []*Data, outBase string) []*Data {
	inputName := outBase + "associationRules.txt"
	var intARs [][]int

	file, err := ioutil.ReadFile(inputName)
	if err != nil {
		return allData
	}

	fileData := fmt.Sprintf("%s", file)
	associationRules := strings.Split(fileData, "\n")

	for _, currRule := range associationRules {
		newIntARs := getIndices(currRule)
		if newIntARs != nil {
			intARs = append(intARs, newIntARs)
		}
	}

	for i, datum := range allData {
		for _, currRule := range intARs {
			newAttribute := 0.0
			//go through and add the datum's example
			for _, index := range currRule {
				featureVal := datum.FeatureSlice[index]
				newAttribute += DecisionTree.GetFloatReflectVal(featureVal)
			}
			//if each attribute value is 1 then we should have a value equal to the length

			allData[i].FeatureSlice = append(allData[i].FeatureSlice, newAttribute)
		}
	}

	return allData
}

func getIndices(rule string) []int {
	var intIndices []int

	indices := strings.Split(rule, ",")

	for _, index := range indices {
		intVal, err := strconv.ParseInt(index, 10, 64)
		if err != nil {
			return nil
		}
		intIndices = append(intIndices, int(intVal))
	}

	return intIndices
}
