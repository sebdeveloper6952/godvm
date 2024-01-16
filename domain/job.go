package domain

type Job struct {
	ID string
}

type JobStatus int

const (
	StatusPaymentRequired  JobStatus = 1
	StatusPaymentCompleted JobStatus = 2
	StatusProcessing       JobStatus = 3
	StatusPartial          JobStatus = 4
	StatusSuccess          JobStatus = 5
	StatusError            JobStatus = 6
)

var (
	JobStatusToString = map[JobStatus]string{
		StatusPaymentRequired: "payment-required",
		StatusProcessing:      "processing",
		StatusSuccess:         "success",
		StatusError:           "error",
		StatusPartial:         "partial",
	}
)

type JobUpdate struct {
	Status         JobStatus
	AmountSats     int
	PaymentRequest string
	Result         string
	FailureMsg     string
}
