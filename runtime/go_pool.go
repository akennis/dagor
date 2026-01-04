package runtime

type IGPool interface {
	Submit(func()) error
	Release()
}
