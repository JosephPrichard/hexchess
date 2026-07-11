package async

type Dispatcher interface {
	Go(func())
}

type SyncDispatcher struct {}

func (_ SyncDispatcher) Go(f func()) {
	f()
}

type AsyncDispatcher struct {}

func (_ AsyncDispatcher) Go(f func()) {
	go f()
}