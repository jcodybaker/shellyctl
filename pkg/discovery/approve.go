package discovery

import (
	"context"
	"sync"
)

type SearchConfirm func(desc string) (approveDevice bool, continueSearch bool, err error)

func newApprover[T any](d *Discoverer, stop chan struct{}) *approver[T] {
	a := &approver[T]{
		ioLock:    &d.ioLock,
		confirmF:  d.searchConfirm,
		toApprove: make(chan *provisionalDevice[T], 100),
		approved:  make(chan *provisionalDevice[T]),
		stop:      stop,
	}
	return a
}

type provisionalDevice[T any] struct {
	device T
	desc   string
}

type approver[T any] struct {
	// ioLock ensures only one approver can read/write to stdio concurrently. This is needed
	// so mDNS/BLE can opperate simultaneously.
	ioLock sync.Locker

	toApprove chan *provisionalDevice[T]
	approved  chan *provisionalDevice[T]

	stop     chan struct{}
	confirmF SearchConfirm

	doneOnce sync.Once
}

func (a *approver[T]) run(ctx context.Context) error {
	defer close(a.approved)
	for {
		select {
		case <-ctx.Done():
			for range a.toApprove {
				// discard anything in toApprove
			}
			return nil
		case d := <-a.toApprove:
			if d == nil {
				return nil
			}
			if err := a.confirm(ctx, d); err != nil {
				return err
			}
		case <-a.stop:
			for range a.toApprove {
				// discard anything in toApprove
			}
			return nil
		}
	}
}

func (a *approver[T]) confirm(ctx context.Context, d *provisionalDevice[T]) error {
	a.ioLock.Lock()
	// We potentially waited while another approver held the io lock; verify ctx hasn't been
	// canceled and the other approver didn't close the stop chan.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.stop:
		return nil
	default:
	}
	var approved, cont bool
	var err error
	if a.confirmF != nil {
		approved, cont, err = a.confirmF(d.desc)
	} else {
		approved = true
		cont = true
	}
	if err != nil {
		a.ioLock.Unlock()
		return err
	}
	if !cont {
		close(a.stop)
	}
	a.ioLock.Unlock()
	if approved {
		select {
		case a.approved <- d:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// getApproved returns an approved device or nil if the approver is complete.
func (a *approver[T]) submit(ctx context.Context, dev T, desc string) bool {
	d := &provisionalDevice[T]{
		device: dev,
		desc:   desc,
	}
	select {
	case a.toApprove <- d:
		return true
	case <-ctx.Done():
		return false
	case <-a.stop:
		return false
	}
}

// getApproved returns an approved device or nil if the approver is complete.
func (a *approver[T]) getApproved(ctx context.Context) T {
	var zeroVal T
	select {
	case d := <-a.approved:
		if d == nil {
			return zeroVal
		}
		return d.device
	case <-ctx.Done():
		// Technically we don't return nil, but the zero value of T, but for T == *someType,
		// that will be nil.
		return zeroVal
	}
}

func (a *approver[T]) done() {
	a.doneOnce.Do(func() { close(a.toApprove) })
}
