package discovery

import (
	"encoding/json"
	"sync"

	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/rs/zerolog/log"
)

type notifications struct {
	statusChan     chan StatusNotification
	fullStatusChan chan StatusNotification
	eventChan      chan EventNotification

	lock sync.Mutex
}

func (n *notifications) register(s mgrpc.MgRPC) {
	log.Debug().Msg("registering notification handlers")
	s.AddHandler("NotifyStatus", n.statusNotificationHandler)
	s.AddHandler("NotifyFullStatus", n.fullStatusNotificationHandler)
	s.AddHandler("NotifyEvent", n.eventNotificationHandler)
}

// StatusNotification carries a status notification and metadata.
type StatusNotification struct {
	Status *shelly.NotifyStatus
	Frame  *frame.Frame
}

type EventNotification struct {
	Event *shelly.NotifyEvent
	Frame *frame.Frame
}

// GetFullStatusNotifications returns a channel which provides NotifyFullStatus messages.
// Messages received before the first invocation of GetFullStatusNotifications will be discarded.
// Consumers MUST be responsive or ther MQTT channel may drop messages.
func (d *Discoverer) GetFullStatusNotifications(buffer int) <-chan StatusNotification {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.fullStatusChan == nil {
		d.fullStatusChan = make(chan StatusNotification, buffer)
	}
	return d.fullStatusChan
}

// GetStatusNotifications returns a channel which provides NotifyStatus messages.
// Messages received before the first invocation of GetStatusNotifications will be discarded.
// Consumers MUST be responsive or ther MQTT channel may drop messages.
func (d *Discoverer) GetStatusNotifications(buffer int) <-chan StatusNotification {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.statusChan == nil {
		d.statusChan = make(chan StatusNotification, buffer)
	}
	return d.statusChan
}

// GetEventNotifications returns a channel which provides events.
// Messages received before the first invocation of GetEventNotifications will be discarded.
// Consumers MUST be responsive or ther MQTT channel may drop messages.
func (d *Discoverer) GetEventNotifications(buffer int) <-chan EventNotification {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.eventChan == nil {
		d.eventChan = make(chan EventNotification, buffer)
	}
	return d.eventChan
}

func (n *notifications) statusNotificationHandler(mr mgrpc.MgRPC, f *frame.Frame) *frame.Frame {
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.statusChan == nil {
		return nil
	}
	s := &shelly.NotifyStatus{}
	if err := json.Unmarshal(f.Params, &s); err != nil {
		log.Err(err).
			Str("src", f.Src).
			Str("dst", f.Dst).
			Int64("id", f.ID).
			Str("method", f.Method).
			Str("payload", string(f.Params)).
			Msg("unmarshalling NotifyStatus frame")
	}
	n.statusChan <- StatusNotification{
		Status: s,
		Frame:  f,
	}
	return nil
}

func (n *notifications) fullStatusNotificationHandler(mr mgrpc.MgRPC, f *frame.Frame) *frame.Frame {
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.fullStatusChan == nil {
		return nil
	}
	s := &shelly.NotifyStatus{}
	if err := json.Unmarshal(f.Params, &s); err != nil {
		log.Err(err).
			Str("src", f.Src).
			Str("dst", f.Dst).
			Int64("id", f.ID).
			Str("method", f.Method).
			Str("payload", string(f.Params)).
			Msg("unmarshalling NotifyFullStatus frame")
	}
	n.fullStatusChan <- StatusNotification{
		Status: s,
		Frame:  f,
	}
	return nil
}

func (n *notifications) eventNotificationHandler(mr mgrpc.MgRPC, f *frame.Frame) *frame.Frame {
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.eventChan == nil {
		return nil
	}
	e := &shelly.NotifyEvent{}
	if err := json.Unmarshal(f.Params, &e); err != nil {
		log.Err(err).
			Str("src", f.Src).
			Str("dst", f.Dst).
			Int64("id", f.ID).
			Str("method", f.Method).
			Str("payload", string(f.Params)).
			Msg("unmarshalling NotifyFullStatus frame")
	}
	n.eventChan <- EventNotification{
		Event: e,
		Frame: f,
	}
	return nil
}
