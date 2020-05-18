package main

import (
	"context"
	"fmt"
	"github.com/kube-sailmaker/k8s-client/client"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
	"time"
)

type JobResult struct {
	Total          int `json:"total"`
	SuccessfulJobs int `json:"successful-jobs"`
	FailedJobs     int `json:"failed-jobs"`

	Deleted int `json:"delete-count"`
	Error   int `json:"error-count"`

	Status string `json:"status"`
}

func (jr *JobResult) toMap() map[string]interface{} {
	return map[string]interface{}{
		"total":           jr.Total,
		"successful_jobs": jr.SuccessfulJobs,
		"failed_jobs":     jr.FailedJobs,
		"deleted":         jr.Deleted,
		"error":           jr.Error,
		"status":          jr.Status,
	}
}

var logger = logrus.New()
var timeout = int64(20)

func main() {
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.999-0700",
	})

	namespace := os.Getenv("JOBS_NAMESPACE")
	if namespace == "" {
		logger.WithField("task", "job-cleanup").Error("using default namespace")
		namespace = "default"
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

	for {
		result := cleanupJob(namespace, successThreshold, failureThreshold)
		logger.WithField("task", "job-cleanup-summary").WithFields(result.toMap()).
			Info(fmt.Sprintf("next cycle in %.0f minutes", frequency.Minutes()))
		time.Sleep(frequency)
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
	} else {
		value = defaultValue
	}
	return value
}

func cleanupJob(namespace string, successThreshold int, failureThreshold int) JobResult {
	k8s := client.GetClient()
	jobInterface := k8s.BatchV1().Jobs(namespace)
	jList, jErr := jobInterface.List(context.TODO(), v1.ListOptions{
		Limit:          10,
		TimeoutSeconds: &timeout,
	})
	var output JobResult
	if jErr != nil {
		logger.Error("error", jErr)
		output = JobResult{
			Status: "error",
		}
		return output
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
			if completionTime == nil {
				completionTime = item.Status.StartTime
			}
			duration := now.Sub(completionTime.Time).Minutes()
			fields := map[string]interface{}{
				"task":      "job-cleanup",
				"name":      item.Name,
				"completed": fmt.Sprintf("%f minutes ago", duration),
			}
			logger.WithFields(fields).Info("check job status")

			successfulJobStatus := item.Status.Succeeded > 0 && float64(successThreshold) < duration
			failureJobStatus := item.Status.Failed > 0 && float64(failureThreshold) < duration
			jobStatus := "SUCCESSFUL"
			if failureJobStatus {
				jobStatus = "FAILED"
			}
			if successfulJobStatus || failureJobStatus {
				propagationPolicy := v1.DeletePropagationBackground
				err := jobInterface.Delete(context.TODO(), item.Name, v1.DeleteOptions{
					PropagationPolicy: &propagationPolicy,
				})
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
	return JobResult{
		Status:         "successful",
		SuccessfulJobs: succeedCount,
		Total:          total,
		Deleted:        deleted,
		FailedJobs:     failed,
		Error:          errorCount,
	}
}
