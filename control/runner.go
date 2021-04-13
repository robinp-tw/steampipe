package control

import (
	"fmt"
	"sync"
	"time"
)

type ControlReportingOptions struct {
	OutputFormat        string
	OutputFileFormats   []string
	OutputFileDirectory string
	WithColor           bool
	WithProgress        bool
}

type ControlType struct {
	ControlID   string `json:"control_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ControlResult struct {
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Resource string `json:"resource"`
}

type ControlRun struct {
	Type    ControlType     `json:"type"`
	Results []ControlResult `json:"results"`
}

type ControlPack struct {
	Timestamp   time.Time    `json:"timestamp"`
	ControlRuns []ControlRun `json:"control_runs"`
}

type ControlEmitter struct {
	listeners []chan ControlPayload
}

type ControlPayload struct {
	Pack    ControlPack
	Options ControlReportingOptions
}

func (e *ControlEmitter) AddListener(ch chan ControlPayload) {
	if e.listeners == nil {
		e.listeners = []chan ControlPayload{}
	}
	e.listeners = append(e.listeners, ch)
}

func (e *ControlEmitter) RemoveListener(ch chan ControlPayload) {
	for i := range e.listeners {
		if e.listeners[i] == ch {
			e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
			break
		}
	}
}

func (e *ControlEmitter) Emit(payload ControlPayload) {
	for _, handler := range e.listeners {
		go func(handler chan ControlPayload) {
			handler <- payload
		}(handler)
	}
}

const (
	ControlAlarm   = "alarm"
	ControlError   = "error"
	ControlInfo    = "info"
	ControlOK      = "ok"
	ControlSkipped = "skipped"
)

func getPluralisedControlsText(count int) string {
	if count == 1 {
		return "control"
	}
	return "controls"
}

func RunControl(reportingOptions ControlReportingOptions) {

	// Generate output formats concurrently
	var wg sync.WaitGroup

	controlEmitter := ControlEmitter{}

	// Add a listener for stdout
	wg.Add(1)
	stdoutChan := make(chan ControlPayload)
	controlEmitter.AddListener(stdoutChan)

	go displayControlResults(stdoutChan, reportingOptions, &wg)

	// Add a listener for each output file
	for _, outputFormat := range reportingOptions.OutputFileFormats {
		wg.Add(1)
		outputChan := make(chan ControlPayload)
		controlEmitter.AddListener(outputChan)
		//formatter := OutputFormatter{
		//	payloadChan: outputChan,
		//	format:      outputFormat,
		//	options:     reportingOptions,
		//	wg:          &wg,
		//}
		fmt.Println("Adding listener")
		go outputFileResults(outputChan, outputFormat, reportingOptions.OutputFileDirectory, &wg)
	}

	// TODO how do I actually get this? Simulate it coming from a channel for now
	controlChan := make(chan ControlPack)
	go runControls(controlChan)
	controlPack := <-controlChan

	controlEmitter.Emit(ControlPayload{controlPack, reportingOptions})

	// Wait for CLI output to complete

	//go displayControlResults(controlPack, reportingOptions, spinner, &wg)

	wg.Wait()
}

func runControls(stream chan ControlPack) {
	select {
	case <-time.After(2 * time.Second):
		controlPack := ControlPack{
			Timestamp: time.Now(),
			ControlRuns: []ControlRun{
				{Type: ControlType{
					ControlID:   "aws.cis.v130.1.1",
					Title:       "Maintain current contact details",
					Description: "Amazon S3 provides a variety of no, or low, cost encryption options to protect data at rest.",
				}, Results: []ControlResult{
					{
						Status: ControlInfo,
					},
				}}, {Type: ControlType{
					ControlID:   "aws.cis.v130.2.1.1",
					Title:       "Ensure all S3 buckets employ encryption-at-rest",
					Description: "Ensure contact email and telephone details for AWS accounts are current and map to more than one individual in your organization. An AWS account supports a number of contact details, and AWS will use these to contact the account owner if activity judged to be in breach of Acceptable Use Policy or indicative of likely security compromise is observed by the AWS Abuse team. Contact details should not be for a single individual, as circumstances may arise where that individual is unavailable. Email contact details should point to a mail alias which forwards email to multiple individuals within the organization; where feasible, phone contact details should point to a PABX hunt group or other call-forwarding system.",
				}, Results: []ControlResult{
					{
						Status: ControlOK,
					},
				}}, {Type: ControlType{
					ControlID:   "aws.cis.v130.2.1.2",
					Title:       "Ensure S3 Bucket Policy allows HTTPS requests",
					Description: "At the Amazon S3 bucket level, you can configure permissions through a bucket policy making the objects accessible only through HTTPS.",
				}, Results: []ControlResult{
					{
						Status: ControlAlarm,
					},
				}}, {Type: ControlType{
					ControlID:   "aws.cis.v130.2.2.1",
					Title:       "Ensure EBS volume encryption is enabled",
					Description: "Elastic Compute Cloud (EC2) supports encryption at rest when using the Elastic Block Store (EBS) service. While disabled by default, forcing encryption at EBS volume creation is supported.",
				}, Results: []ControlResult{
					{
						Status: ControlOK,
					},
				}},
			},
		}
		stream <- controlPack
	}
}