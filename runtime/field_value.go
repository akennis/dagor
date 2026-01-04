package runtime

// FieldValue is a value of a field in a vertex.
// It is used to store the runtime value of a field produced by an operator during graph execution.
type FieldValue struct {
	Name  string // operator field name
	Value any    // runtime value
}
