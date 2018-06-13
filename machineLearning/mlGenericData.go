package ritaML

//Data Basic data type that can hold any supervised learner type
type Data struct {
	Class        int
	Prediction   int
	UID          string
	FeatureSlice []interface{}
}
