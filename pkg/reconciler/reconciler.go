package reconciler

type Reconciler interface {
	Reconcile(string) error
}
