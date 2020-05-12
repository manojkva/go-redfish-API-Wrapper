package idrac

import (
	"context"
	"fmt"
	//"os"
	"strings"
	"time"

	RFWrap "github.com/manojkva/go-redfish-api-wrapper/pkg/redfishwrap"
	redfish "opendev.org/airship/go-redfish/client"
)

type IdracRedfishClient struct {
	Username  string
	Password  string
	HostIP    string
	IDRAC_ver string
}

func (a *IdracRedfishClient) createContext() context.Context {

	var auth = redfish.BasicAuth{UserName: a.Username,
		Password: a.Password,
	}
	ctx := context.WithValue(context.Background(), redfish.ContextBasicAuth, auth)
	return ctx
}

func (a *IdracRedfishClient) UpgradeFirmware(filelocation string) {

	ctx := a.createContext()

	httpPushURI := RFWrap.UpdateService(ctx, a.HostIP)

	fmt.Printf("%v", httpPushURI)

	etag := RFWrap.GetETagHttpURI(ctx, a.HostIP)
	fmt.Printf("%v", etag)
	imageURI, _ := RFWrap.HTTPUriDownload(ctx, a.HostIP, filelocation, etag)

	fmt.Printf("%v", imageURI)

	jobID := RFWrap.SimpleUpdateRequest(ctx, a.HostIP, imageURI)

	fmt.Printf("%v", jobID)

	a.CheckJobStatus(jobID)
}

func (a *IdracRedfishClient) CheckJobStatus(jobId string) bool {
	ctx := a.createContext()
	start := time.Now()
	var result bool = false

       if  jobId == ""{
           fmt.Println("Job ID is null. Returing Failed Job Status")
           return false
       }

	for {

		statusCode, jobInfo := RFWrap.GetTask(ctx, a.HostIP, jobId)

		timeelapsedInMinutes := time.Since(start).Minutes()

		if (statusCode == 202) || (statusCode == 200) {
			fmt.Printf("HTTP  status OK")

		} else {
			fmt.Printf("Failed to check the status")
		    return false
		}

		if timeelapsedInMinutes >= 5 {
			fmt.Println("\n- FAIL: Timeout of 5 minute has been hit, update job should of already been marked completed. Check the iDRAC job queue and LC logs to debug the issue")
			return true
		} else if jobInfo.Messages != nil {
		if strings.Contains(jobInfo.Messages[0].Message, "failed") {
			fmt.Println("FAIL")
			return false

		} else if strings.Contains(jobInfo.Messages[0].Message, "scheduled") {
			//	fmt.Prinln("\n- PASS, job ID %s successfully marked as scheduled, powering on or rebooting the server to apply the update" % data[u"Id"] ")
			result = true
			break

		} else if strings.Contains(jobInfo.Messages[0].Message, "completed successfully") {
			//		fmt.Prinln("\n- PASS, job ID %s successfully marked as scheduled, powering on or rebooting the server to apply the update" % data[u"Id"] ")
			fmt.Println("Success")
			result = true
			break
		}
		}else {
			time.Sleep(time.Second*5)
			continue
		}
	}
	return result
}

func (a *IdracRedfishClient) RebootServer(systemID string) bool {

	ctx := a.createContext()

	//Systems/System.Embedded.1/Actions/ComputerSystem.Reset
	resetRequestBody := redfish.ResetRequestBody { ResetType: redfish.RESETTYPE_FORCE_RESTART }

	return RFWrap.ResetServer(ctx, a.HostIP, systemID, resetRequestBody)

}

func (a *IdracRedfishClient) PowerOn(systemID string) bool {
	ctx := a.createContext()
	resetRequestBody := redfish.ResetRequestBody { ResetType: redfish.RESETTYPE_ON }

	return RFWrap.ResetServer(ctx, a.HostIP, systemID, resetRequestBody)

}

func (a *IdracRedfishClient) PowerOff(systemID string) bool {
	ctx := a.createContext()
	resetRequestBody := redfish.ResetRequestBody { ResetType: redfish.RESETTYPE_GRACEFUL_SHUTDOWN }

	return RFWrap.ResetServer(ctx, a.HostIP, systemID, resetRequestBody)
}

func (a *IdracRedfishClient) GetVirtualMediaStatus(managerID string, media string) bool {
	ctx := a.createContext()
	return RFWrap.GetVirtualMediaConnectedStatus(ctx, a.HostIP, managerID, media)
}

func (a *IdracRedfishClient) EjectISO(managerID string, media string) bool {
	ctx := a.createContext()
	return RFWrap.EjectVirtualMedia(ctx, a.HostIP, managerID, media)
}

func (a *IdracRedfishClient) SetOneTimeBoot(systemID string) bool {
	ctx := a.createContext()
	computeSystem := redfish.ComputerSystem{Boot: redfish.Boot{BootSourceOverrideEnabled: "Once"}}

	return RFWrap.SetSystem(ctx, a.HostIP, systemID, computeSystem)

}

func (a *IdracRedfishClient) InsertISO(managerID string, mediaID string, imageURL string) bool {

	ctx := a.createContext()

	if a.GetVirtualMediaStatus(managerID, mediaID) {
		fmt.Println("Exiting .. Already connected")
		return false
	}
	insertMediaReqBody := redfish.InsertMediaRequestBody{
		Image: imageURL,
	}
	return RFWrap.InsertVirtualMedia(ctx, a.HostIP, managerID, mediaID, insertMediaReqBody)

}

func (a *IdracRedfishClient) GetVirtualDisks(systemID string, controllerID string)[]string {

	ctx := a.createContext()
	idrefs := RFWrap.GetVolumes(ctx, a.HostIP, systemID, controllerID)
	if idrefs == nil{
		return nil
	}
	virtualDisks := []string{}
	for _,id := range idrefs{

		fmt.Printf("VirtualDisk Info %v\n", id.OdataId)
		vd := strings.Split(id.OdataId,"/")
		if vd != nil {
		  virtualDisks = append(virtualDisks,vd[len(vd)-1])
		}
	}
	return virtualDisks

}

func (a *IdracRedfishClient) DeletVirtualDisk(systemID string, storageID string) string {
	ctx := a.createContext()

	return RFWrap.DeleteVirtualDisk(ctx, a.HostIP, systemID, storageID)
}

func (a *IdracRedfishClient) CreateVirtualDisk(systemID string, controllerID string, volumeType string, name string, urilist []string) string {
	ctx := a.createContext()

	drives := []redfish.IdRef{}

	for _, uri := range urilist {
		driveinfo := fmt.Sprintf("/redfish/v1/Systems/%s/Storage/Drives/%s",systemID, uri)
		drives = append(drives, redfish.IdRef{OdataId: driveinfo})
	}

	createvirtualBodyReq := redfish.CreateVirtualDiskRequestBody{
		VolumeType: redfish.VolumeType(volumeType),
		Name:       name,
		Drives:     drives,
	}

	return RFWrap.CreateVirtualDisk(ctx, a.HostIP, systemID, controllerID, createvirtualBodyReq)
}

func (a *IdracRedfishClient)CleanVirtualDisksIfAny(systemID string, controllerID string) bool{

	var result bool = false

	// Get the list of VirtualDisks
	virtualDisks := a.GetVirtualDisks(systemID, controllerID)
	totalvirtualDisks := len(virtualDisks)
	var countofVDcreated int = 0
	// for testing skip the OS Disk
	//virtualDisks = virtualDisks[1:] 
	if totalvirtualDisks == 0 {
		fmt.Printf("No existing RAID disks found")
		result = true
	} else {
		for _,vd  := range virtualDisks {
			jobid  := a.DeletVirtualDisk(systemID,vd)
			fmt.Printf("Delete Job ID %v\n",jobid)
			result = a.CheckJobStatus(jobid)

			if result == false {
				fmt.Printf("Failed to delete virtual disk %v\n",vd)
				return result
			}
                        time.Sleep(time.Second*10) //Sleep in between calls
			countofVDcreated += 1

		}
	}
	if countofVDcreated  != totalvirtualDisks {
		result  = false
	}

    return result
}

func (a * IdracRedfishClient)GetNodeUUID(systemID string )(string, bool){

	ctx := a.createContext()
	computerSystem, _  := RFWrap.GetSystem(ctx,a.HostIP,systemID)

	if computerSystem != nil{
		return computerSystem.UUID, true
	}
	return "", false
}

func (a * IdracRedfishClient)GetPowerStatus(systemID string ) bool{

	ctx := a.createContext()
	computerSystem, _  := RFWrap.GetSystem(ctx,a.HostIP,systemID)

	if computerSystem != nil{
                if  computerSystem.PowerState  ==  "On"{
                    return  true
               }
	}
	return false
}
