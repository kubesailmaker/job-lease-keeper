package main

import (
	"context"
	"fmt"
	"github.com/kube-sailmaker/job-lease-keeper/k8s/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
	"time"
	"github.com/sirupsen/logrus"
)

type JobResult struct {
	Total int
	SuccessfulJobs int
	FailedJobs int

	Deleted int
	Error int

	Status string
}

var logger = logrus.New()

func main() {
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.999-0700",
	})

	namespace := os.Getenv("JOBS_NAMESPACE")
	if namespace == "" {
		logger.WithField("task", "job-cleanup").Error("provide namespace using JOBS_NAMESPACE")
		os.Exit(1)
	}

	successThreshold := getIntFromEnv("JOBS_SUCCESS_THRESHOLD_MINUTES", 60)
	delayFrequency := getIntFromEnv("CHECK_FREQUENCY_MINUTES", 30)
	failureThreshold := getIntFromEnv("JOBS_FAILURE_THRESHOLD_MINUTES", 120)

	logger.WithField("task", "job-cleanup").
		WithField("job_cleanup_threshold", fmt.Sprintf("%d minutes", successThreshold)).
		WithField("check_frequency", fmt.Sprintf("%d minutes", delayFrequency)).
		WithField("namespace", namespace).
		Info("configuration")

	frequency := time.Duration(delayFrequency) * time.Minute
	logger.WithFields(map[string]interface{}{
		"task":      "job-cleanup",
		"frequency": fmt.Sprintf("%d minutes", frequency),
	})
	done := make(chan JobResult)
	go cleanupJob(namespace, successThreshold, failureThreshold, done)
	for {
		time.AfterFunc(frequency, func() {
			progress := make(chan JobResult)
			go cleanupJob(namespace, successThreshold, failureThreshold, progress)
		})
	}
}

func getIntFromEnv(envName string, defaultValue int) int {
	var value int
	thresholdMin := os.Getenv(envName)
	if thresholdMin != "" {
		tValue, convErr := strconv.Atoi(thresholdMin)
		if convErr != nil {
			logger.Warn("defaulting to ", defaultValue)
			value = defaultValue
		} else {
			value = tValue
		}
	}
	return value
}

func cleanupJob(namespace string, successThreshold int, failureThreshold int, output chan JobResult) {
	k8s := client.GetClient()
	jList, jErr := k8s.BatchV1().Jobs(namespace).List(context.TODO(), v1.ListOptions{
	})
	if jErr != nil {
		logger.Error("error", jErr)
		output <- JobResult{
		  Status: "error",
		}
		close(output)
	}
	total := len(jList.Items)
	now := time.Now()
	succeedCount := 0
	errorCount := 0
	deleted := 0
	failed := 0
	for _, item := range jList.Items {
		if item.Status.Active == 0 && item.Status.Succeeded > 0 || item.Status.Failed > 0 {
			succeedCount = succeedCount + 1
			completionTime := item.Status.CompletionTime
			duration := now.Sub(completionTime.Time).Minutes()
			jobStatus := item.Status.String()
			fields := map[string]interface{}{
				"task":      "job-cleanup",
				"name":      item.Name,
				"status":    jobStatus,
				"completed": fmt.Sprintf("%f minutes ago", duration),
			}
			logger.WithFields(fields).Info("check job status")

			successfulJobStatus := item.Status.Succeeded > 0 && float64(successThreshold) < duration
			failureJobStatus := item.Status.Failed > 0 && float64(failureThreshold) < duration

			if  successfulJobStatus || failureJobStatus {
				err := k8s.BatchV1().Jobs(namespace).Delete(context.TODO(), item.Name, v1.DeleteOptions{})
				resultLog := logger.WithFields(fields).WithField("action", "clean")
				if err != nil {
					resultLog.
						WithField("job_status", jobStatus).
						WithField("delete", "FAIL").Info("clean completed jobs failed")
					errorCount += 1
				} else {
					resultLog.
						WithField("job_status", jobStatus).
						WithField("delete", "SUCCESS").Info("clean completed jobs succeeded")
					deleted += 1
				}
			}
		}
	}
	output <- JobResult{
		Status: "successful",
		SuccessfulJobs: succeedCount,
		Total: total,
		Deleted: deleted,
		FailedJobs: failed,
		Error: errorCount,
	}
}
