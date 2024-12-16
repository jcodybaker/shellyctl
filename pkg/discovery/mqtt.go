package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
)

func (d *Discoverer) MQTTConnect(ctx context.Context) error {
	ll := d.logCtx(ctx, "mqtt")
	if d.mqttClientOptions == nil {
		ll.Debug().Msg("no MQTT servers defined; skipping mqtt connect")
		return nil
	}
	// opts.SetConnectionLostHandler(c.onConnectionLost)
	ll.Info().Str("broker", d.mqttClientOptions.Servers[0].String()).Msg("connecting to MQTT Broker")
	d.mqttClient = mqtt.NewClient(d.mqttClientOptions)

	token := d.mqttClient.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connect error: %w", err)
	}

	for _, t := range d.mqttTopicSubs {
		c, err := newMQTTConsumer(ctx, t, d.mqttClient)
		if err != nil {
			return fmt.Errorf("subscribing to MQTT topic %q: %w", t, err)
		}
		s := mgrpc.Serve(ctx, c)
		s.AddHandler("NotifyStatus", d.statusNotificationHandler)
		s.AddHandler("NotifyFullStatus", d.fullStatusNotificationHandler)
		s.AddHandler("NotifyEvent", d.eventNotificationHandler)
	}
	return nil
}

// searchMQTT finds new devices via MQTT.
func (d *Discoverer) searchMQTT(ctx context.Context, stop chan struct{}) ([]*Device, error) {
	// 1) Subscribe to /shellies/announce
	// 2) Subscribe to +/online
	// 3)

	// 1) Subscribe to '+/status'
	// 2) Subscribe to '+/online'; remove 'off' devices when lost
	// 3) wait discovery timeout EXACTLY once
	// 3) return current known device list.
	stopSearch := new(atomic.Bool)
	ll := d.logCtx(ctx, "mqtt")
	if !d.mqttSearchEnabled {
		return nil, nil
	}

	c := make(chan *shelly.ShellyGetDeviceInfoResponse, mdnsSearchBuffer)

	d.mqttClient.Subscribe("shellies/announce", 1, func(_ mqtt.Client, m mqtt.Message) {
		var deviceInfo shelly.ShellyGetDeviceInfoResponse
		if err := json.Unmarshal(m.Payload(), &deviceInfo); err != nil {
			ll.Err(err).
				Uint16("message_id", m.MessageID()).
				Str("topic", m.Topic()).
				Msg("parsing MQTT message as device info")
			return
		}
		if stopSearch.Load() {
			ll.Warn().
				Uint16("message_id", m.MessageID()).
				Str("topic", m.Topic()).
				Str("device_id", deviceInfo.ID).
				Msg("discarding late MQTT search response")
			return
		}
		ll.Debug().
			Uint16("message_id", m.MessageID()).
			Str("topic", m.Topic()).
			Str("device_id", deviceInfo.ID).
			Msg("got MQTT search response")
		c <- &deviceInfo
	})

	approver := newApprover[*shelly.ShellyGetDeviceInfoResponse](d, stop)
	defer approver.done()

	workerLimiter := make(chan struct{}, d.concurrency)
	defer close(workerLimiter)
	var outputLock sync.Mutex
	var output []*Device

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer approver.done()
		for deviceInfo := range c {
			desc := fmt.Sprintf(
				"mqtt device %q (%s/%s)",
				deviceInfo.ID,
				deviceInfo.App,
				deviceInfo.Model,
			)
			approver.submit(ctx, deviceInfo, desc)
		}
	}()

	// Display the confirmation dialogues if necessary, then pass devices to the approved queue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := approver.run(ctx); err != nil {
			ll.Err(err).Msg("confirming devices")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			deviceInfo := approver.getApproved(ctx)
			if deviceInfo == nil {
				return
			}
			wg.Add(1)
			// Occupy a space in the workerLimiter buffer or block until one is available.
			workerLimiter <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-workerLimiter }()
				if dev := d.processMQTTAnnounceResponse(ctx, deviceInfo); dev != nil {
					outputLock.Lock()
					defer outputLock.Unlock()
					output = append(output, dev)
				}
			}()
		}
	}()

	token := d.mqttClient.Publish("shellies/command", 1, false, []byte("announce"))
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("publishing search message to mqtt: %w", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(d.searchTimeout):
	}

	// We can't guarantee that the mqtt has coallesed and processed all incoming messages. So it's difficult
	// be certain we can close the channel. The atomic stopSearch makes this safer, but it's not a guarantee.
	stopSearch.Store(true)
	token = d.mqttClient.Unsubscribe("shellies/announce")
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("unsubscribing from mqtt search message responses: %w", err)
	}
	close(c)

	wg.Wait()
	return output, nil
}

func (d *Discoverer) processMQTTAnnounceResponse(ctx context.Context, deviceInfo *shelly.ShellyGetDeviceInfoResponse) *Device {
	ll := d.logCtx(ctx, "mqtt").With().
		Str("mqtt_id", deviceInfo.ID).
		Logger()

	switch deviceInfo.Gen {
	case "2", "3":
		// ok
	default:
		ll.Warn().Str("gen", deviceInfo.Gen.String()).Msg("unsupport device `gen`; skipping")
		return nil
	}

	dev, err := d.AddMQTTDevice(ctx, deviceInfo.ID)
	if err != nil {
		ll.Warn().Err(err).Msg("failed to add mqtt device")
	}
	return dev
}
