package config

import "encoding/json"

type GraphConfig struct {
	Name     string                   `json:"name"`
	Vertices map[string]*VertexConfig `json:"vertices"`
}

// OnError is the action to take when an error occurs.
const (
	OnErrorStop     string = "stop"     // stop the graph execution when an vertex error occurs.
	OnErrorContinue string = "continue" // continue the graph execution when an vertex error occurs.
)

type VertexConfig struct {
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`

	// input fields map. operator field name -> vertex field name.
	Inputs map[string]string `json:"inputs"`
	// output fields map. operator field name -> vertex field name.
	Outputs map[string]string `json:"outputs"`

	// on error action.
	// default is "stop".
	OnError string `json:"on_error"`
}
