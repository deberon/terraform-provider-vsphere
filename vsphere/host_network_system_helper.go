package vsphere

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// hostNetworkSystemFromHostSystem locates a HostNetworkSystem from a specified
// HostSystem.
func hostNetworkSystemFromHostSystem(hs *object.HostSystem) (*object.HostNetworkSystem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultAPITimeout)
	defer cancel()
	return hs.ConfigManager().NetworkSystem(ctx)
}

// hostNetworkSystemFromHostSystemID locates a HostNetworkSystem from a
// specified HostSystem managed object ID.
func hostNetworkSystemFromHostSystemID(client *govmomi.Client, hsID string) (*object.HostNetworkSystem, error) {
	hs, err := hostSystemFromID(client, hsID)
	if err != nil {
		return nil, err
	}
	return hostNetworkSystemFromHostSystem(hs)
}

// hostVSwitchFromName locates a virtual switch on the supplied
// HostNetworkSystem by name.
func hostVSwitchFromName(client *govmomi.Client, ns *object.HostNetworkSystem, name string) (*types.HostVirtualSwitch, error) {
	var mns mo.HostNetworkSystem
	pc := client.PropertyCollector()
	ctx, cancel := context.WithTimeout(context.Background(), defaultAPITimeout)
	defer cancel()
	if err := pc.RetrieveOne(ctx, ns.Reference(), []string{"networkInfo.vswitch"}, &mns); err != nil {
		return nil, fmt.Errorf("error fetching host network properties: %s", err)
	}

	for _, sw := range mns.NetworkInfo.Vswitch {
		if sw.Name == name {
			return &sw, nil
		}
	}

	return nil, fmt.Errorf("could not find virtual switch %s", name)
}

// hostPortGroupFromName locates a port group on the supplied HostNetworkSystem
// by name.
func hostPortGroupFromName(client *govmomi.Client, ns *object.HostNetworkSystem, name string) (*types.HostPortGroup, error) {
	var mns mo.HostNetworkSystem
	pc := client.PropertyCollector()
	ctx, cancel := context.WithTimeout(context.Background(), defaultAPITimeout)
	defer cancel()
	if err := pc.RetrieveOne(ctx, ns.Reference(), []string{"networkInfo.portgroup"}, &mns); err != nil {
		return nil, fmt.Errorf("error fetching host network properties: %s", err)
	}

	for _, pg := range mns.NetworkInfo.Portgroup {
		if pg.Spec.Name == name {
			return &pg, nil
		}
	}

	return nil, fmt.Errorf("could not find port group %s", name)
}

// networkProperties gets the properties for a specific Network.
//
// The Network type usually represents a standard port group in vCenter - it
// has been set up on a host or a set of hosts, and is usually configured via
// through an appropriate HostNetworkSystem. vCenter, however, groups up these
// networks and displays them as a single network that VM can use across hosts,
// facilitating HA and vMotion for VMs that use standard port groups versus DVS
// port groups. Hence the "Network" object is mainly a read-only MO and is only
// useful for checking some very base level attributes.
func networkProperties(net *object.Network) (*mo.Network, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultAPITimeout)
	defer cancel()
	var props mo.Network
	if err := net.Properties(ctx, net.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// networkObjectFromHostSystem locates the network object in vCenter for a
// specific HostSystem and network name.
//
// It does this by searching for all networks in the folder hierarchy that
// match the given network name for the HostSystem's managed object reference
// ID. This match is returned - if nothing is found, an error is given.
func networkObjectFromHostSystem(client *govmomi.Client, hs *object.HostSystem, name string) (*object.Network, error) {
	// Validate vCenter as this function is only relevant there
	if err := validateVirtualCenter(client); err != nil {
		return nil, err
	}
	finder := find.NewFinder(client.Client, false)
	ctx, cancel := context.WithTimeout(context.Background(), defaultAPITimeout)
	defer cancel()
	nets, err := finder.NetworkList(ctx, "*/"+name)
	if err != nil {
		return nil, err
	}

	for _, n := range nets {
		net, ok := n.(*object.Network)
		if !ok {
			// Not a standard port group (possibly DVS, etc), pass
			continue
		}
		props, err := networkProperties(net)
		if err != nil {
			return nil, err
		}
		for _, hsRef := range props.Host {
			if hsRef.Value == hs.Reference().Value {
				// This is our network
				return net, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find a matching %q on host ID %q", name, hs.Reference().Value)
}
