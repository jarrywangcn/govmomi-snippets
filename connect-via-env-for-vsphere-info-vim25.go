//
// Go code to connect to vSphere via environment
// variables and retrieve the defautl datacenter
//
// -- Cormac J. Hogan (VMware)
//
// -- 25 Jan 2021
//
//------------------------------------------------------------------------------------------------------------------------------------
//
// Client information from Doug MacEachern:
//
// govmomi.Client extends vim25.Client
// govmomi.Client does nothing extra aside from automatic login
//
// In the early days (2015), govmomi.Client did much more, but we moved most of it to vim25.Client.
// govmomi.Client remained for compatibility and minor convenience.
//
// Using soap.Client and vim25.Client directly allows apps to use other authentication methods,
// session caching, session keepalive, retries, fine grained TLS configuration, etc.
//
// For the inventory, ContainerView is a vSphere primitive.
// Compared to Finder, ContainerView tends to use less round trip calls to vCenter.
// It may generate more response data however.
//
// Finder was written for govc, where we treat the vSphere inventory as a virtual filesystem.
// The inventory path as input to `govc` behaves similar to the `ls` command, with support for relative paths, wildcard matching, etc.
//
// Use govc commands as a reference, and "godoc" for examples that can be run against `vcsim`:
// See: https://godoc.org/github.com/vmware/govmomi/view#pkg-examples
//
//------------------------------------------------------------------------------------------------------------------------------------
//
// functionality comes from the following packages
//
//    context        - https://golang.org/pkg/context/
//    flag           - https://golang.org/pkg/flag/
//    fmt            - https://golang.org/pkg/fmt/
//    net/url        - https://golang.org/pkg/net/url/
//    os             - https://golang.org/pkg/os/
//    text/tabwriter - https://golang.org/pkg/text/tabwriter/
//
//    govmomi        - https://github.com/vmware/govmomi


package main
  
import (
        "context"
        "fmt"
        "net/url"
	"os"
	"text/tabwriter"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/units"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/session/cache"
)

func main() {


	// We need to get 3 environment variables in order to connect to the vSphere infra
	//
	// GOVMOMI_URL
	// GOVMOMI_USERNAME
	// GOVMOMI_PASSWORD
	//

	vc := os.Getenv ("GOVMOMI_URL")
	user := os.Getenv ("GOVMOMI_USERNAME")
	pwd := os.Getenv ("GOVMOMI_PASSWORD")


	fmt.Printf ("DEBUG: vc is %s\n", vc)	
	fmt.Printf ("DEBUG: user is %s\n", user)	
	fmt.Printf ("DEBUG: password is %s\n", pwd)	

//
// Imagine that there were multiple operations taking place such as processing some data, logging into vCenter, etc. 
// If one of the operations failed, the context would be used to share the fact that all of the other operations sharing that context needs cancelling. 
//

        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

//
// Create a vSphere/vCenter client
//
//    The govmomi client requires a URL object, u, not just a string representation of the vCenter URL.

	u, err := soap.ParseURL(vc)

	if u == nil {
		fmt.Println("could not parse URL (environment variables set?)")
	}

	if err != nil {
        	fmt.Println("URL parsing not successful, error %v", err)
                return
        }

	u.User = url.UserPassword(user, pwd)

//
// Ripped from https://github.com/vmware/govmomi/blob/master/examples/examples.go
//

// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: true,
	}

//    c - Return the client object c 
//    err - Return the error object err
//    ctx - Pass in the shared context

	c := new (vim25.Client)

	err = s.Login(ctx, c, nil)

	if err != nil {
        	fmt.Println("Log in not successful- could not get vCenter client: %v", err)
                return
        } else {
        	fmt.Println("Log in successful")

//
// Create a view manager
//

		m := view.NewManager(c)

//
// Create a container view of HostSystem objects
//

		v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)

		if err != nil {
			fmt.Printf("Unable to create Host Container View: error %s", err)
			return
		}

		defer v.Destroy(ctx)

//
// Retrieve summary property for all hosts
// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.HostSystem.html
//

		var hss []mo.HostSystem

		err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"summary"}, &hss)

		if err != nil {
			fmt.Printf("Unable to retrieve Host information: error %s", err)
			return
		}

//
// Print summary per host (see also: govc/host/info.go)
//

		tw := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Printf("\n*** Host Information ***\n")
		fmt.Printf("------------------------\n\n")
		fmt.Fprintf(tw, "Name:\tUsed CPU:\tTotal CPU:\tFree CPU:\tUsed Memory:\tTotal Memory:\tFree Memory:\t\n")

		for _, hs := range hss {
			totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
			freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
			freeMemory := int64(hs.Summary.Hardware.MemorySize) - (int64(hs.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024)
			fmt.Fprintf(tw, "%s\t", hs.Summary.Config.Name)
			fmt.Fprintf(tw, "%d\t", hs.Summary.QuickStats.OverallCpuUsage)
			fmt.Fprintf(tw, "%d\t", totalCPU)
			fmt.Fprintf(tw, "%d\t", freeCPU)
			fmt.Fprintf(tw, "%s\t", (units.ByteSize(hs.Summary.QuickStats.OverallMemoryUsage))*1024*1024)
			fmt.Fprintf(tw, "%s\t", units.ByteSize(hs.Summary.Hardware.MemorySize))
			fmt.Fprintf(tw, "%d\t", freeMemory)
			fmt.Fprintf(tw, "\n")
		}

		_ = tw.Flush()

//
// Create a container view of Datastore objects
//

		v2, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)

		if err != nil {
			fmt.Printf("Unable to create Datastore Container View: error %s", err)
			return
		}

		defer v2.Destroy(ctx)

//
// Retrieve summary property for all datastores
// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.Datastore.html
//

		var dss []mo.Datastore
		err = v2.Retrieve(ctx, []string{"Datastore"}, []string{"summary"}, &dss)

		if err != nil {
			fmt.Printf("Unable to retrieve Datastore information: error %s", err)
			return
		}

//
// Print summary per datastore (see also: govc/datastore/info.go)
//

		tw2 := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Printf("\n*** Datastore Information ***\n")
		fmt.Printf("------------------------------\n\n")
		fmt.Fprintf(tw2, "Name:\tType:\tCapacity:\tFree:\n")

		for _, ds := range dss {
			fmt.Fprintf(tw2, "%s\t", ds.Summary.Name)
			fmt.Fprintf(tw2, "%s\t", ds.Summary.Type)
			fmt.Fprintf(tw2, "%s\t", units.ByteSize(ds.Summary.Capacity))
			fmt.Fprintf(tw2, "%s\t", units.ByteSize(ds.Summary.FreeSpace))
			fmt.Fprintf(tw2, "\n")
		}

		_ = tw2.Flush()


//
// Create a container view of VM objects
//


		v3, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
		if err != nil {
			fmt.Printf("Unable to create Virtual Machine Container View: error %s", err)
			return
		}

		defer v3.Destroy(ctx)

//
// Retrieve summary property for all machines
// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.VirtualMachine.html
//

		var vms []mo.VirtualMachine
		err = v3.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms)

		if err != nil {
			fmt.Printf("Unable to retrieve VM information: error %s", err)
			return
		}

//
// Print summary per vm (see also: govc/vm/info.go)
//

		tw3 := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Printf("\n*** VM Information ***\n")
		fmt.Printf("-----------------------\n\n")
		fmt.Fprintf(tw3, "Name:\tGuest Full Name:\t\n")

		for _, vm := range vms {
			fmt.Fprintf(tw3, "%s:\t", vm.Summary.Config.Name)
			fmt.Fprintf(tw3, "%s\t\n", vm.Summary.Config.GuestFullName)
		}

		fmt.Fprintf(tw3, "\n")

		_ = tw3.Flush()

	}
}
