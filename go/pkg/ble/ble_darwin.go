package ble

import (
	"context"
	"sync"

	"github.com/JuulLabs-OSS/cbgo"
)

func scan(ctx context.Context) (<-chan Peripheral, error) {
	cm := cbgo.NewCentralManager(nil)
	ch := make(chan Peripheral)
	delegate := &centralManagerDelegate{scanCh: ch}
	cm.SetDelegate(delegate)
	delegate.WaitForPowerOn()

	cm.Scan(nil, &cbgo.CentralManagerScanOpts{AllowDuplicates: false})
	go func() {
		select {
		case <-ctx.Done():
			cm.StopScan()
		}
		close(ch)
	}()

	return ch, nil
}

type centralManagerDelegate struct {
	state     cbgo.ManagerState
	mu        sync.Mutex
	observers []chan struct{}

	scanCh chan<- Peripheral
}

func (c *centralManagerDelegate) WaitForPowerOn() {
	if c.state == cbgo.ManagerStatePoweredOn {
		return
	}

	ch := make(chan struct{})
	c.mu.Lock()
	c.observers = append(c.observers, ch)
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		for i := range c.observers {
			if c.observers[i] == ch {
				c.observers = append(c.observers[:i], c.observers[i+1:]...)
				break
			}
		}
		c.mu.Unlock()
	}()

	for {
		<-ch
		if c.state == cbgo.ManagerStatePoweredOn {
			return
		}
	}
}

func (c *centralManagerDelegate) DidConnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral) {
	panic("implement me")
}

func (c *centralManagerDelegate) DidDisconnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral, err error) {
	panic("implement me")
}

func (c *centralManagerDelegate) DidFailToConnectPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral, err error) {
	panic("implement me")
}

func (c *centralManagerDelegate) DidDiscoverPeripheral(cmgr cbgo.CentralManager, prph cbgo.Peripheral, advFields cbgo.AdvFields, rssi int) {
	c.scanCh <- Peripheral{
		RSSI:             int16(rssi),
		Address:          prph.Identifier().String(),
		Name:             advFields.LocalName,
		ManufacturerData: advFields.ManufacturerData,
	}
}

func (c *centralManagerDelegate) CentralManagerDidUpdateState(cmgr cbgo.CentralManager) {
	c.state = cmgr.State()
	c.mu.Lock()
	observers := c.observers
	c.mu.Unlock()

	for _, v := range observers {
		select {
		case v <- struct{}{}:
		default:
		}
	}
}

func (c *centralManagerDelegate) CentralManagerWillRestoreState(cmgr cbgo.CentralManager, opts cbgo.CentralManagerRestoreOpts) {
	panic("implement me")
}
