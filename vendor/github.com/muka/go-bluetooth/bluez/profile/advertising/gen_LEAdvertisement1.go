// Code generated DO NOT EDIT

package advertising



import (
   "sync"
   "github.com/muka/go-bluetooth/bluez"
   "github.com/muka/go-bluetooth/util"
   "github.com/muka/go-bluetooth/props"
   "github.com/godbus/dbus/v5"
)

var LEAdvertisement1Interface = "org.bluez.LEAdvertisement1"


// NewLEAdvertisement1 create a new instance of LEAdvertisement1
//
// Args:
// - objectPath: freely definable
func NewLEAdvertisement1(objectPath dbus.ObjectPath) (*LEAdvertisement1, error) {
	a := new(LEAdvertisement1)
	a.client = bluez.NewClient(
		&bluez.Config{
			Name:  "org.bluez",
			Iface: LEAdvertisement1Interface,
			Path:  dbus.ObjectPath(objectPath),
			Bus:   bluez.SystemBus,
		},
	)
	
	a.Properties = new(LEAdvertisement1Properties)

	_, err := a.GetProperties()
	if err != nil {
		return nil, err
	}
	
	return a, nil
}


/*
LEAdvertisement1 LE Advertisement Data hierarchy

Specifies the Advertisement Data to be broadcast and some advertising
parameters.  Properties which are not present will not be included in the
data.  Required advertisement data types will always be included.
All UUIDs are 128-bit versions in the API, and 16 or 32-bit
versions of the same UUID will be used in the advertising data as appropriate.

*/
type LEAdvertisement1 struct {
	client     				*bluez.Client
	propertiesSignal 	chan *dbus.Signal
	objectManagerSignal chan *dbus.Signal
	objectManager       *bluez.ObjectManager
	Properties 				*LEAdvertisement1Properties
	watchPropertiesChannel chan *dbus.Signal
}

// LEAdvertisement1Properties contains the exposed properties of an interface
type LEAdvertisement1Properties struct {
	lock sync.RWMutex `dbus:"ignore"`

	/*
	Type Determines the type of advertising packet requested.

			Possible values: "broadcast" or "peripheral"
	*/
	Type string

	/*
	Discoverable Advertise as general discoverable. When present this
			will override adapter Discoverable property.

			Note: This property shall not be set when Type is set
			to broadcast.
	*/
	Discoverable bool

	/*
	LocalName Local name to be used in the advertising report. If the
			string is too big to fit into the packet it will be
			truncated.

			If this property is available 'local-name' cannot be
			present in the Includes.
	*/
	LocalName string

	/*
	Appearance Appearance to be used in the advertising report.

			Possible values: as found on GAP Service.
	*/
	Appearance uint16

	/*
	SolicitUUIDs Array of UUIDs to include in "Service Solicitation"
			Advertisement Data.
	*/
	SolicitUUIDs []string

	/*
	Includes List of features to be included in the advertising
			packet.

			Possible values: as found on
					LEAdvertisingManager.SupportedIncludes
	*/
	Includes []string

	/*
	ManufacturerData Manufactuer Data fields to include in
			the Advertising Data.  Keys are the Manufacturer ID
			to associate with the data.
	*/
	ManufacturerData map[uint16]interface{}

	/*
	ServiceData Service Data elements to include. The keys are the
			UUID to associate with the data.
	*/
	ServiceData map[string]interface{}

	/*
	Data Advertising Type to include in the Advertising
			Data. Key is the advertising type and value is the
			data as byte array.

			Note: Types already handled by other properties shall
			not be used.

			Possible values:
				<type> <byte array>
				...

			Example:
				<Transport Discovery> <Organization Flags...>
				0x26                   0x01         0x01...
	*/
	Data map[byte]interface{}

	/*
	SecondaryChannel 
	*/
	SecondaryChannel string `dbus:"omitEmpty"`

	/*
	ServiceUUIDs List of UUIDs to include in the "Service UUID" field of
			the Advertising Data.
	*/
	ServiceUUIDs []string

	/*
	DiscoverableTimeout The discoverable timeout in seconds. A value of zero
			means that the timeout is disabled and it will stay in
			discoverable/limited mode forever.

			Note: This property shall not be set when Type is set
			to broadcast.
	*/
	DiscoverableTimeout uint16

	/*
	Duration Duration of the advertisement in seconds. If there are
			other applications advertising no duration is set the
			default is 2 seconds.
	*/
	Duration uint16

	/*
	Timeout Timeout of the advertisement in seconds. This defines
			the lifetime of the advertisement.
	*/
	Timeout uint16

}

//Lock access to properties
func (p *LEAdvertisement1Properties) Lock() {
	p.lock.Lock()
}

//Unlock access to properties
func (p *LEAdvertisement1Properties) Unlock() {
	p.lock.Unlock()
}




// SetType set Type value
func (a *LEAdvertisement1) SetType(v string) error {
	return a.SetProperty("Type", v)
}



// GetType get Type value
func (a *LEAdvertisement1) GetType() (string, error) {
	v, err := a.GetProperty("Type")
	if err != nil {
		return "", err
	}
	return v.Value().(string), nil
}




// SetDiscoverable set Discoverable value
func (a *LEAdvertisement1) SetDiscoverable(v bool) error {
	return a.SetProperty("Discoverable", v)
}



// GetDiscoverable get Discoverable value
func (a *LEAdvertisement1) GetDiscoverable() (bool, error) {
	v, err := a.GetProperty("Discoverable")
	if err != nil {
		return false, err
	}
	return v.Value().(bool), nil
}




// SetLocalName set LocalName value
func (a *LEAdvertisement1) SetLocalName(v string) error {
	return a.SetProperty("LocalName", v)
}



// GetLocalName get LocalName value
func (a *LEAdvertisement1) GetLocalName() (string, error) {
	v, err := a.GetProperty("LocalName")
	if err != nil {
		return "", err
	}
	return v.Value().(string), nil
}




// SetAppearance set Appearance value
func (a *LEAdvertisement1) SetAppearance(v uint16) error {
	return a.SetProperty("Appearance", v)
}



// GetAppearance get Appearance value
func (a *LEAdvertisement1) GetAppearance() (uint16, error) {
	v, err := a.GetProperty("Appearance")
	if err != nil {
		return uint16(0), err
	}
	return v.Value().(uint16), nil
}




// SetSolicitUUIDs set SolicitUUIDs value
func (a *LEAdvertisement1) SetSolicitUUIDs(v []string) error {
	return a.SetProperty("SolicitUUIDs", v)
}



// GetSolicitUUIDs get SolicitUUIDs value
func (a *LEAdvertisement1) GetSolicitUUIDs() ([]string, error) {
	v, err := a.GetProperty("SolicitUUIDs")
	if err != nil {
		return []string{}, err
	}
	return v.Value().([]string), nil
}




// SetIncludes set Includes value
func (a *LEAdvertisement1) SetIncludes(v []string) error {
	return a.SetProperty("Includes", v)
}



// GetIncludes get Includes value
func (a *LEAdvertisement1) GetIncludes() ([]string, error) {
	v, err := a.GetProperty("Includes")
	if err != nil {
		return []string{}, err
	}
	return v.Value().([]string), nil
}




// SetManufacturerData set ManufacturerData value
func (a *LEAdvertisement1) SetManufacturerData(v map[string]interface{}) error {
	return a.SetProperty("ManufacturerData", v)
}



// GetManufacturerData get ManufacturerData value
func (a *LEAdvertisement1) GetManufacturerData() (map[string]interface{}, error) {
	v, err := a.GetProperty("ManufacturerData")
	if err != nil {
		return map[string]interface{}{}, err
	}
	return v.Value().(map[string]interface{}), nil
}




// SetServiceData set ServiceData value
func (a *LEAdvertisement1) SetServiceData(v map[string]interface{}) error {
	return a.SetProperty("ServiceData", v)
}



// GetServiceData get ServiceData value
func (a *LEAdvertisement1) GetServiceData() (map[string]interface{}, error) {
	v, err := a.GetProperty("ServiceData")
	if err != nil {
		return map[string]interface{}{}, err
	}
	return v.Value().(map[string]interface{}), nil
}




// SetData set Data value
func (a *LEAdvertisement1) SetData(v map[string]interface{}) error {
	return a.SetProperty("Data", v)
}



// GetData get Data value
func (a *LEAdvertisement1) GetData() (map[string]interface{}, error) {
	v, err := a.GetProperty("Data")
	if err != nil {
		return map[string]interface{}{}, err
	}
	return v.Value().(map[string]interface{}), nil
}




// SetSecondaryChannel set SecondaryChannel value
func (a *LEAdvertisement1) SetSecondaryChannel(v string) error {
	return a.SetProperty("SecondaryChannel", v)
}



// GetSecondaryChannel get SecondaryChannel value
func (a *LEAdvertisement1) GetSecondaryChannel() (string, error) {
	v, err := a.GetProperty("SecondaryChannel")
	if err != nil {
		return "", err
	}
	return v.Value().(string), nil
}




// SetServiceUUIDs set ServiceUUIDs value
func (a *LEAdvertisement1) SetServiceUUIDs(v []string) error {
	return a.SetProperty("ServiceUUIDs", v)
}



// GetServiceUUIDs get ServiceUUIDs value
func (a *LEAdvertisement1) GetServiceUUIDs() ([]string, error) {
	v, err := a.GetProperty("ServiceUUIDs")
	if err != nil {
		return []string{}, err
	}
	return v.Value().([]string), nil
}




// SetDiscoverableTimeout set DiscoverableTimeout value
func (a *LEAdvertisement1) SetDiscoverableTimeout(v uint16) error {
	return a.SetProperty("DiscoverableTimeout", v)
}



// GetDiscoverableTimeout get DiscoverableTimeout value
func (a *LEAdvertisement1) GetDiscoverableTimeout() (uint16, error) {
	v, err := a.GetProperty("DiscoverableTimeout")
	if err != nil {
		return uint16(0), err
	}
	return v.Value().(uint16), nil
}




// SetDuration set Duration value
func (a *LEAdvertisement1) SetDuration(v uint16) error {
	return a.SetProperty("Duration", v)
}



// GetDuration get Duration value
func (a *LEAdvertisement1) GetDuration() (uint16, error) {
	v, err := a.GetProperty("Duration")
	if err != nil {
		return uint16(0), err
	}
	return v.Value().(uint16), nil
}




// SetTimeout set Timeout value
func (a *LEAdvertisement1) SetTimeout(v uint16) error {
	return a.SetProperty("Timeout", v)
}



// GetTimeout get Timeout value
func (a *LEAdvertisement1) GetTimeout() (uint16, error) {
	v, err := a.GetProperty("Timeout")
	if err != nil {
		return uint16(0), err
	}
	return v.Value().(uint16), nil
}



// Close the connection
func (a *LEAdvertisement1) Close() {
	
	a.unregisterPropertiesSignal()
	
	a.client.Disconnect()
}

// Path return LEAdvertisement1 object path
func (a *LEAdvertisement1) Path() dbus.ObjectPath {
	return a.client.Config.Path
}

// Client return LEAdvertisement1 dbus client
func (a *LEAdvertisement1) Client() *bluez.Client {
	return a.client
}

// Interface return LEAdvertisement1 interface
func (a *LEAdvertisement1) Interface() string {
	return a.client.Config.Iface
}

// GetObjectManagerSignal return a channel for receiving updates from the ObjectManager
func (a *LEAdvertisement1) GetObjectManagerSignal() (chan *dbus.Signal, func(), error) {

	if a.objectManagerSignal == nil {
		if a.objectManager == nil {
			om, err := bluez.GetObjectManager()
			if err != nil {
				return nil, nil, err
			}
			a.objectManager = om
		}

		s, err := a.objectManager.Register()
		if err != nil {
			return nil, nil, err
		}
		a.objectManagerSignal = s
	}

	cancel := func() {
		if a.objectManagerSignal == nil {
			return
		}
		a.objectManagerSignal <- nil
		a.objectManager.Unregister(a.objectManagerSignal)
		a.objectManagerSignal = nil
	}

	return a.objectManagerSignal, cancel, nil
}


// ToMap convert a LEAdvertisement1Properties to map
func (a *LEAdvertisement1Properties) ToMap() (map[string]interface{}, error) {
	return props.ToMap(a), nil
}

// FromMap convert a map to an LEAdvertisement1Properties
func (a *LEAdvertisement1Properties) FromMap(props map[string]interface{}) (*LEAdvertisement1Properties, error) {
	props1 := map[string]dbus.Variant{}
	for k, val := range props {
		props1[k] = dbus.MakeVariant(val)
	}
	return a.FromDBusMap(props1)
}

// FromDBusMap convert a map to an LEAdvertisement1Properties
func (a *LEAdvertisement1Properties) FromDBusMap(props map[string]dbus.Variant) (*LEAdvertisement1Properties, error) {
	s := new(LEAdvertisement1Properties)
	err := util.MapToStruct(s, props)
	return s, err
}

// ToProps return the properties interface
func (a *LEAdvertisement1) ToProps() bluez.Properties {
	return a.Properties
}

// GetWatchPropertiesChannel return the dbus channel to receive properties interface
func (a *LEAdvertisement1) GetWatchPropertiesChannel() chan *dbus.Signal {
	return a.watchPropertiesChannel
}

// SetWatchPropertiesChannel set the dbus channel to receive properties interface
func (a *LEAdvertisement1) SetWatchPropertiesChannel(c chan *dbus.Signal) {
	a.watchPropertiesChannel = c
}

// GetProperties load all available properties
func (a *LEAdvertisement1) GetProperties() (*LEAdvertisement1Properties, error) {
	a.Properties.Lock()
	err := a.client.GetProperties(a.Properties)
	a.Properties.Unlock()
	return a.Properties, err
}

// SetProperty set a property
func (a *LEAdvertisement1) SetProperty(name string, value interface{}) error {
	return a.client.SetProperty(name, value)
}

// GetProperty get a property
func (a *LEAdvertisement1) GetProperty(name string) (dbus.Variant, error) {
	return a.client.GetProperty(name)
}

// GetPropertiesSignal return a channel for receiving udpdates on property changes
func (a *LEAdvertisement1) GetPropertiesSignal() (chan *dbus.Signal, error) {

	if a.propertiesSignal == nil {
		s, err := a.client.Register(a.client.Config.Path, bluez.PropertiesInterface)
		if err != nil {
			return nil, err
		}
		a.propertiesSignal = s
	}

	return a.propertiesSignal, nil
}

// Unregister for changes signalling
func (a *LEAdvertisement1) unregisterPropertiesSignal() {
	if a.propertiesSignal != nil {
		a.propertiesSignal <- nil
		a.propertiesSignal = nil
	}
}

// WatchProperties updates on property changes
func (a *LEAdvertisement1) WatchProperties() (chan *bluez.PropertyChanged, error) {
	return bluez.WatchProperties(a)
}

func (a *LEAdvertisement1) UnwatchProperties(ch chan *bluez.PropertyChanged) error {
	return bluez.UnwatchProperties(a, ch)
}




/*
Release 
			This method gets called when the service daemon
			removes the Advertisement. A client can use it to do
			cleanup tasks. There is no need to call
			UnregisterAdvertisement because when this method gets
			called it has already been unregistered.


*/
func (a *LEAdvertisement1) Release() error {
	
	return a.client.Call("Release", 0, ).Store()
	
}

