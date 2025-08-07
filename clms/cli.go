package clms

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/cheynewallace/tabby"

	"github.com/quantifyearth/reclaimer/internal/utils"
)

func inspectAllGeneratedData() error {
	index, err := FetchIndexGeneratedData()
	if nil != err {
		return err
	}

	for _, item := range index {
		fmt.Printf("%s: %s", item.UID, item.Title)
		if items, ok := item.Downloads["items"]; ok {
			fmt.Printf(" (%d items)", len(items))
		}
		fmt.Printf("\n")
	}

	return nil
}

func inspectGeneratadData(UID string) error {
	index, err := FetchIndexGeneratedData()
	if nil != err {
		return err
	}

	for _, item := range index {
		if item.UID == UID {
			fmt.Printf("title: %s\n", item.Title)
			fmt.Printf("description: %s\n", item.Description)

			if items, ok := item.Downloads["items"]; ok {
				for _, item := range items {
					fmt.Printf("\t%s: %s\n", item.ID, item.FullPath)
				}
			}

			break
		}
	}

	return nil
}

func inspectAllPrepackagedData() error {
	index, err := FetchIndexPrepackagedData()
	if nil != err {
		return err
	}

	for _, item := range index {
		fmt.Printf("%s: %s", item.UID, item.Title)
		fmt.Printf(" (%d items)", len(item.Files.Items))
		fmt.Printf("\n")
	}

	return nil
}

func inspectPrepackagedData(UID string) error {
	index, err := FetchIndexPrepackagedData()
	if nil != err {
		return err
	}

	for _, item := range index {
		if item.UID == UID {
			fmt.Printf("title: %s\n", item.Title)
			fmt.Printf("description: %s\n", item.Description)

			for _, item := range item.Files.Items {
				fmt.Printf("\t%s: %s (%s)\n", item.ID, item.File, item.Size)
			}

			break
		}
	}

	return nil
}

func completeDownload(sessionToken string, taskID string, extract bool, outputPath string) error {
	var status CLMSTaskStatus
	var err error
	for {
		status, err = GetTaskStatus(taskID, sessionToken)
		if nil != err {
			return fmt.Errorf("error checking task status: %w", err)
		}

		if status.Status == TaskInProgress {
			fmt.Printf("In progress...")
			time.Sleep(time.Second * 5)
			continue
		}
		if status.Status != TaskFinished {
			return fmt.Errorf("received unexpected status: %s", status.Status)
		}
		break
	}

	if "" == status.DownloadURL {
		return fmt.Errorf("got an empty download URL for task")
	}
	downloadURL, err := url.Parse(status.DownloadURL)
	if nil != err {
		return fmt.Errorf("failed to parse download url: %w", err)
	}
	targetFilename := path.Base(downloadURL.Path)

	fmt.Printf("Downloading data...")
	return utils.DownloadFile(status.DownloadURL, targetFilename, extract, outputPath)
}

func fetchGeneratedData(
	uid string,
	downloadID string,
	extract bool,
	outputFormat string,
	coordinateSystem string,
	sessionToken string,
	outputPath string,
) error {

	task, err := RequestGeneratedData(uid, downloadID, outputFormat, coordinateSystem, sessionToken, outputPath)
	if nil != err {
		return err
	}

	// we only asked for one thing, so if there's an error task, it's game over
	if len(task.ErrorTaskIDs) > 0 {
		return fmt.Errorf("only got error for tasks.")
	}
	if len(task.TaskIDs) != 1 {
		return fmt.Errorf("expected one task, got %d", len(task.TaskIDs))
	}
	fmt.Printf("Data requested...")

	taskID := task.TaskIDs[0].ID
	return completeDownload(sessionToken, taskID, extract, outputPath)
}

func fetchPrepackagedData(
	uid string,
	downloadID string,
	extract bool,
	sessionToken string,
	outputPath string,
) error {
	task, err := RequestPrepackagedData(uid, downloadID, sessionToken, outputPath)
	if nil != err {
		return err
	}

	// we only asked for one thing, so if there's an error task, it's game over
	if len(task.ErrorTaskIDs) > 0 {
		return fmt.Errorf("only got error for tasks.")
	}
	if len(task.TaskIDs) != 1 {
		return fmt.Errorf("expected one task, got %d", len(task.TaskIDs))
	}
	fmt.Printf("Data requested...")

	taskID := task.TaskIDs[0].ID
	return completeDownload(sessionToken, taskID, extract, outputPath)
}

func directDownload(
	uid string,
	downloadID string,
	extract bool,
	sessionToken string,
	outputPath string,
) error {

	directLinks, err := RequestDirectData(uid, downloadID, sessionToken, outputPath)
	if nil != err {
		return err
	}
	fmt.Printf("Received %d URLs.\n", len(directLinks))

	// we have to assume outputPath is a directory in this case, so make it so
	if "" != outputPath {
		err = os.MkdirAll(outputPath, os.ModePerm)
		if nil != err {
			return fmt.Errorf("failed to create output dir: %w", err)
		}
	}

	for _, urlstr := range directLinks {
		fmt.Printf("Downloading %s...\n", urlstr)
		url, err := url.Parse(urlstr)
		if nil != err {
			return fmt.Errorf("failed to parse url: %w", err)
		}
		err = utils.DownloadFile(urlstr, path.Base(url.Path), extract, outputPath)
		if nil != err {
			return fmt.Errorf("failed to download: %w", err)
		}
	}
	return nil
}

//----- VERBS

type verb func([]string) error

func searchVerb(args []string) error {
	flag := flag.NewFlagSet("clms", flag.ExitOnError)
	var (
		prepackaged = flag.Bool("prepackaged", false, "Search prepackaged data")
		UID         = flag.String("uid", "", "UID of resource.")
	)
	flag.Parse(args)

	if (nil == UID) || (nil == prepackaged) {
		// stop the static analyser being upset
		panic("Flags didn't work")
	}

	var err error
	if "" == *UID {
		if *prepackaged {
			err = inspectAllPrepackagedData()
		} else {
			err = inspectAllGeneratedData()
		}
	} else {
		if *prepackaged {
			err = inspectPrepackagedData(*UID)
		} else {
			err = inspectGeneratadData(*UID)
		}
	}
	return err
}

func downloadVerb(args []string) error {
	flag := flag.NewFlagSet("clms", flag.ExitOnError)
	var (
		prepackaged = flag.Bool("prepackaged", false, "Search prepackaged data")
		UID         = flag.String("uid", "", "UID of resource.")
		downloadID  = flag.String("download_id", "", "The ID of the actual item within the resource to fetch.")
		apiKeyPath  = flag.String("apikeyfile", "", "Path of JSON API key downloaded from CLMS account page.")
		extract     = flag.Bool("extract", false, "If item is compressed extract automatically")
		output      = flag.String("output", "", "Destination name (filename for single item, directory name if multiple).")
		format      = flag.String("format", "Geotiff", "Requested download format. Defaults to GeoTIFF.")
		coordSystem = flag.String("cgs", "EPSG:4326", "Global coordinate System to use. Defaults to EPSG:4326.")
	)
	flag.Parse(args)

	if (nil == UID) || (nil == apiKeyPath) || (nil == output) || (nil == extract) || (nil == downloadID) || (nil == format) || (nil == coordSystem) {
		// stop the static analyser being upset
		panic("Flags didn't work")
	}

	if "" == *apiKeyPath {
		return fmt.Errorf("No API key provided, required for downloads.")
	}
	apiKey, err := LoadAPIKey(*apiKeyPath)
	if nil != err {
		return fmt.Errorf("failed to load api key: %w", err)
	}
	sessionToken, err := apiKey.GetSessionToken()
	if nil != err {
		return fmt.Errorf("failed to get session token: %w", err)
	}

	if *prepackaged {
		if ("Geotiff" != *format) || ("EPSG:4326" != *coordSystem) {
			return fmt.Errorf("Can not specify format or coordinate system for prepackaged CLMS data.")
		}
		err = fetchPrepackagedData(*UID, *downloadID, *extract, sessionToken, *output)
	} else {
		err = fetchGeneratedData(*UID, *downloadID, *extract, *format, *coordSystem, sessionToken, *output)
	}
	return err
}

func requestsVerb(args []string) error {
	flag := flag.NewFlagSet("clms", flag.ExitOnError)
	var (
		apiKeyPath = flag.String("apikeyfile", "", "Path of JSON API key downloaded from CLMS account page.")
	)
	flag.Parse(args)

	if nil == apiKeyPath {
		// stop the static analyser being upset
		panic("Flags didn't work")
	}

	if "" == *apiKeyPath {
		return fmt.Errorf("No API key provided, required for downloads.")
	}
	apiKey, err := LoadAPIKey(*apiKeyPath)
	if nil != err {
		return fmt.Errorf("failed to load api key: %w", err)
	}
	sessionToken, err := apiKey.GetSessionToken()
	if nil != err {
		return fmt.Errorf("failed to get session token: %w", err)
	}

	statuses, err := GetRequests(sessionToken)
	if nil != err {
		return fmt.Errorf("failed to get requests: %w", err)
	}
	{
		s, _ := json.Marshal(statuses)
		fmt.Printf("%v\n", string(s))
	}

	t := tabby.New()
	t.AddHeader("Request ID", "Dataset ID", "Status")

	for taskID, status := range statuses {
		for _, dataset := range status.Datasets {
			t.AddLine(taskID, dataset.DatasetID, status.Status)
		}
	}
	t.Print()

	return nil
}

func resumeVerb(args []string) error {
	flag := flag.NewFlagSet("clms", flag.ExitOnError)
	var (
		apiKeyPath = flag.String("apikeyfile", "", "Path of JSON API key downloaded from CLMS account page.")
		requestID  = flag.String("request", "", "Request made via API earlier.")
		extract    = flag.Bool("extract", false, "If item is compressed extract automatically")
		output     = flag.String("output", "", "Destination name (filename for single item, directory name if multiple).")
	)
	flag.Parse(args)

	if (nil == apiKeyPath) || (nil == requestID) || (nil == extract) || (nil == output) {
		// stop the static analyser being upset
		panic("Flags didn't work")
	}

	if "" == *requestID {
		return fmt.Errorf("Request ID required")
	}

	if "" == *apiKeyPath {
		return fmt.Errorf("No API key provided, required for downloads.")
	}
	apiKey, err := LoadAPIKey(*apiKeyPath)
	if nil != err {
		return fmt.Errorf("failed to load api key: %w", err)
	}
	sessionToken, err := apiKey.GetSessionToken()
	if nil != err {
		return fmt.Errorf("failed to get session token: %w", err)
	}

	return completeDownload(sessionToken, *requestID, *extract, *output)
}

func directVerb(args []string) error {
	flag := flag.NewFlagSet("clms", flag.ExitOnError)
	var (
		apiKeyPath = flag.String("apikeyfile", "", "Path of JSON API key downloaded from CLMS account page.")
		UID        = flag.String("uid", "", "UID of resource.")
		downloadID = flag.String("download_id", "", "The ID of the actual item within the resource to fetch.")
		extract    = flag.Bool("extract", false, "If item is compressed extract automatically")
		output     = flag.String("output", "", "Destination name (filename for single item, directory name if multiple).")
	)
	flag.Parse(args)

	if (nil == apiKeyPath) || (nil == UID) || (nil == extract) || (nil == output) || (nil == downloadID) {
		// stop the static analyser being upset
		panic("Flags didn't work")
	}

	if "" == *UID {
		return fmt.Errorf("Datset ID required")
	}

	if "" == *apiKeyPath {
		return fmt.Errorf("No API key provided, required for downloads.")
	}
	apiKey, err := LoadAPIKey(*apiKeyPath)
	if nil != err {
		return fmt.Errorf("failed to load api key: %w", err)
	}
	sessionToken, err := apiKey.GetSessionToken()
	if nil != err {
		return fmt.Errorf("failed to get session token: %w", err)
	}

	return directDownload(*UID, *downloadID, *extract, sessionToken, *output)
}

func CLMSMain(args []string) {

	var subcommands = map[string]verb{
		"search":   searchVerb,
		"download": downloadVerb,
		"requests": requestsVerb,
		"resume":   resumeVerb,
		"direct":   directVerb,
	}

	if len(args) == 0 {
		fmt.Printf("Supported verbs are:\n")
		for cmd := range subcommands {
			fmt.Fprintf(os.Stderr, "\t%s\n", cmd)
		}
		return
	}

	cmd, args := args[0], args[1:]

	if subcmd, ok := subcommands[cmd]; ok {
		err := subcmd(args)
		if nil != err {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Unrecognised verb. Options are:\n")
		for cmd := range subcommands {
			fmt.Fprintf(os.Stderr, "\t%s\n", cmd)
		}
		os.Exit(1)
	}
}
