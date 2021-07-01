// Code generated DO NOT EDIT

package advertising



import (
   "sync"
   "github.com/muka/go-bluetooth/bluez"
   "github.com/muka/go-bluetooth/util"
   "github.com/muka/go-bluetooth/props"
   "github.com/godbus/dbus/v5"
   "fmt"
)

var LEAdvertisingManager1Interface = "org.bluez.LEAdvertisingManager1"


// NewLEAdvertisingManager1 create a new instance of LEAdvertisingManager1
//
// Args:
// - objectPath: /org/bluez/{hci0,hci1,...}
func NewLEAdvertisingManager1(objectPath dbus.ObjectPath) (*LEAdvertisingManager1, error) {
	a := new(LEAdvertisingManager1)
	a.client = bluez.NewClient(
		&bluez.Config{
			Name:  "org.bluez",
			Iface: LEAdvertisingManager1Interface,
			Path:  dbus.ObjectPath(objectPath),
			Bus:   bluez.SystemBus,
		},
	)
	
	a.Properties = new(LEAdvertisingManager1Properties)

	_, err := a.GetProperties()
	if err != nil {
		return nil, err
	}
	
	return a, nil
}

// NewLEAdvertisingManager1FromAdapterID create a new instance of LEAdvertisingManager1
// adapterID: ID of an adapter eg. hci0
func NewLEAdvertisingManager1FromAdapterID(adapterID string) (*LEAdvertisingManager1, error) {
	a := new(LEAdvertisingManager1)
	a.client = bluez.NewClient(
		&bluez.Config{
			Name:  "org.bluez",
			Iface: LEAdvertisingManager1Interface,
			Path:  dbus.ObjectPath(fmt.Sprintf("/org/bluez/%s", adapterID)),
			Bus:   bluez.SystemBus,
		},
	)
	
	a.Properties = new(LEAdvertisingManager1Properties)

	_, err := a.GetProperties()
	if err != nil {
		return nil, err
	}
	
	return a, nil
}


/*
LEAdvertisingManager1 LE Advertising Manager hierarchy

The Advertising Manager allows external applications to register Advertisement
Data which should be broadcast to devices.  Advertisement Data elements must
follow the API for LE Advertisement Data described above.

*/
type LEAdvertisingManager1 struct {
	client     				*bluez.Client
	propertiesSignal 	chan *dbus.Signal
	objectManagerSignal chan *dbus.Signal
	objectManager       *bluez.ObjectManager
	Properties 				*LEAdvertisingManager1Properties
	watchPropertiesChannel chan *dbus.Signal
}

// LEAdvertisingManager1Properties contains the exposed properties of an interface
type LEAdvertisingManager1Properties struct {
	lock sync.RWMutex `dbus:"ignore"`

	/*
	ActiveInstances Number of active advertising instances.
	*/
	ActiveInstances byte

	/*
	SupportedInstances Number of available advertising instances.
	*/
	SupportedInstances byte

	/*
	SupportedIncludes List of supported system includes.

			Possible values: "tx-power"
					 "appearance"
					 "local-name"
	*/
	SupportedIncludes []string

}

//Lock access to properties
func (p *LEAdvertisingManager1Properties) Lock() {
	p.lock.Lock()
}

//Unlock access to properties
func (p *LEAdvertisingManager1Properties) Unlock() {
	p.lock.Unlock()
}




// SetActiveInstances set ActiveInstances value
func (a *LEAdvertisingManager1) SetActiveInstances(v byte) error {
	return a.SetProperty("ActiveInstances", v)
}



// GetActiveInstances get ActiveInstances value
func (a *LEAdvertisingManager1) GetActiveInstances() (byte, error) {
	v, err := a.GetProperty("ActiveInstances")
	if err != nil {
		return byte(0), err
	}
	return v.Value().(byte), nil
}




// SetSupportedInstances set SupportedInstances value
func (a *LEAdvertisingManager1) SetSupportedInstances(v byte) error {
	return a.SetProperty("SupportedInstances", v)
}



// GetSupportedInstances get SupportedInstances value
func (a *LEAdvertisingManager1) GetSupportedInstances() (byte, error) {
	v, err := a.GetProperty("SupportedInstances")
	if err != nil {
		return byte(0), err
	}
	return v.Value().(byte), nil
}




// SetSupportedIncludes set SupportedIncludes value
func (a *LEAdvertisingManager1) SetSupportedIncludes(v []string) error {
	return a.SetProperty("SupportedIncludes", v)
}



// GetSupportedIncludes get SupportedIncludes value
func (a *LEAdvertisingManager1) GetSupportedIncludes() ([]string, error) {
	v, err := a.GetProperty("SupportedIncludes")
	if err != nil {
		return []string{}, err
	}
	return v.Value().([]string), nil
}



// Close the connection
func (a *LEAdvertisingManager1) Close() {
	
	a.unregisterPropertiesSignal()
	
	a.client.Disconnect()
}

// Path return LEAdvertisingManager1 object path
func (a *LEAdvertisingManager1) Path() dbus.ObjectPath {
	return a.client.Config.Path
}

// Client return LEAdvertisingManager1 dbus client
func (a *LEAdvertisingManager1) Client() *bluez.Client {
	return a.client
}

// Interface return LEAdvertisingManager1 interface
func (a *LEAdvertisingManager1) Interface() string {
	return a.client.Config.Iface
}

// GetObjectManagerSignal return a channel for receiving updates from the ObjectManager
func (a *LEAdvertisingManager1) GetObjectManagerSignal() (chan *dbus.Signal, func(), error) {

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


// ToMap convert a LEAdvertisingManager1Properties to map
func (a *LEAdvertisingManager1Properties) ToMap() (map[string]interface{}, error) {
	return props.ToMap(a), nil
}

// FromMap convert a map to an LEAdvertisingManager1Properties
func (a *LEAdvertisingManager1Properties) FromMap(props map[string]interface{}) (*LEAdvertisingManager1Properties, error) {
	props1 := map[string]dbus.Variant{}
	for k, val := range props {
		props1[k] = dbus.MakeVariant(val)
	}
	return a.FromDBusMap(props1)
}

// FromDBusMap convert a map to an LEAdvertisingManager1Properties
func (a *LEAdvertisingManager1Properties) FromDBusMap(props map[string]dbus.Variant) (*LEAdvertisingManager1Properties, error) {
	s := new(LEAdvertisingManager1Properties)
	err := util.MapToStruct(s, props)
	return s, err
}

// ToProps return the properties interface
func (a *LEAdvertisingManager1) ToProps() bluez.Properties {
	return a.Properties
}

// GetWatchPropertiesChannel return the dbus channel to receive properties interface
func (a *LEAdvertisingManager1) GetWatchPropertiesChannel() chan *dbus.Signal {
	return a.watchPropertiesChannel
}

// SetWatchPropertiesChannel set the dbus channel to receive properties interface
func (a *LEAdvertisingManager1) SetWatchPropertiesChannel(c chan *dbus.Signal) {
	a.watchPropertiesChannel = c
}

// GetProperties load all available properties
func (a *LEAdvertisingManager1) GetProperties() (*LEAdvertisingManager1Properties, error) {
	a.Properties.Lock()
	err := a.client.GetProperties(a.Properties)
	a.Properties.Unlock()
	return a.Properties, err
}

// SetProperty set a property
func (a *LEAdvertisingManager1) SetProperty(name string, value interface{}) error {
	return a.client.SetProperty(name, value)
}

// GetProperty get a property
func (a *LEAdvertisingManager1) GetProperty(name string) (dbus.Variant, error) {
	return a.client.GetProperty(name)
}

// GetPropertiesSignal return a channel for receiving udpdates on property changes
func (a *LEAdvertisingManager1) GetPropertiesSignal() (chan *dbus.Signal, error) {

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
func (a *LEAdvertisingManager1) unregisterPropertiesSignal() {
	if a.propertiesSignal != nil {
		a.propertiesSignal <- nil
		a.propertiesSignal = nil
	}
}

// WatchProperties updates on property changes
func (a *LEAdvertisingManager1) WatchProperties() (chan *bluez.PropertyChanged, error) {
	return bluez.WatchProperties(a)
}

func (a *LEAdvertisingManager1) UnwatchProperties(ch chan *bluez.PropertyChanged) error {
	return bluez.UnwatchProperties(a, ch)
}




/*
RegisterAdvertisement 
			Registers an advertisement object to be sent over the LE
			Advertising channel.  The service must be exported
			under interface LEAdvertisement1.

			InvalidArguments error indicates that the object has
			invalid or conflicting properties.

			InvalidLength error indicates that the data
			provided generates a data packet which is too long.

			The properties of this object are parsed when it is
			registered, and any changes are ignored.

			If the same object is registered twice it will result in
			an AlreadyExists error.

			If the maximum number of advertisement instances is
			reached it will result in NotPermitted error.

			Possible errors: org.bluez.Error.InvalidArguments
					 org.bluez.Error.AlreadyExists
					 org.bluez.Error.InvalidLength

*/
func (a *LEAdvertisingManager1) RegisterAdvertisement(advertisement dbus.ObjectPath, options map[string]interface{}) error {
	
	return a.client.Call("RegisterAdvertisement", 0, advertisement, options).Store()
	
}

/*
UnregisterAdvertisement 
			This unregisters an advertisement that has been
			previously registered.  The object path parameter must
			match the same value that has been used on registration.

			Possible errors: org.bluez.Error.InvalidArguments
					 org.bluez.Error.DoesNotExist


*/
func (a *LEAdvertisingManager1) UnregisterAdvertisement(advertisement dbus.ObjectPath) error {
	
	return a.client.Call("UnregisterAdvertisement", 0, advertisement).Store()
	
}

