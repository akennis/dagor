package operator

import (
	"sync"
)

// OperatorPool is a pool for operators.
// It is used to reduce the memory allocation and GC overhead.
type OperatorPool struct {
	name string
	pool *sync.Pool
}

// NewOperatorPool creates a new operator pool.
func NewOperatorPool(name string, opBuilder func() IOperator) *OperatorPool {
	return &OperatorPool{
		name: name,
		pool: &sync.Pool{
			New: func() any {
				return opBuilder()
			},
		},
	}
}

// Get gets an operator from the pool.
func (p *OperatorPool) Get() IOperator {
	return p.pool.Get().(IOperator)
}

// Put puts an operator back to the pool.
func (p *OperatorPool) Put(op IOperator) {
	p.pool.Put(op)
}
