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

var logger = logrus.New()

func main() {
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.999-0700",
	})

	var olderThanMinutes = float64(120)
	var delayFrequency = int64(30)

	namespace := os.Getenv("JOBS_NAMESPACE")
	if namespace == "" {
		logger.WithField("task", "job-cleanup").Error("provide namespace using JOBS_NAMESPACE")
		os.Exit(1)
	}

	thresholdMin := os.Getenv("JOBS_COMPLETION_THRESHOLD_MINUTES")
	if thresholdMin != "" {
		tValue, convErr := strconv.Atoi(thresholdMin)
		if convErr != nil {
			logger.Warn("defaulting to ", olderThanMinutes)
		} else {
			olderThanMinutes = float64(tValue)
		}
	}

	delayFrequencyValue := os.Getenv("CHECK_FREQUENCY_MINUTES")
	if delayFrequencyValue != "" {
		fValue, convErr := strconv.Atoi(delayFrequencyValue)
		if convErr != nil {
			logger.Warn("defaulting to", delayFrequency)
		} else {
			delayFrequency = int64(fValue)
		}
	}
	logger.WithField("task", "job-cleanup").
		WithField("job_cleanup_threshold", fmt.Sprintf("%f minutes", olderThanMinutes)).
		WithField("check_frequency", fmt.Sprintf("%d minutes", delayFrequency)).
		WithField("namespace", namespace).
		Info("configuration")

	frequency := time.Duration(delayFrequency) * time.Minute
	logger.WithFields(map[string]interface{}{
		"task":      "job-cleanup",
		"frequency": fmt.Sprintf("%d minutes", frequency),
	})
	cleanupJob(namespace, olderThanMinutes)
	for {
		time.AfterFunc(frequency, func() {
			cleanupJob(namespace, olderThanMinutes)
		})
	}
}

func cleanupJob(namespace string, thresholdMinutes float64) {
	k8s := client.GetClient()
	jList, jErr := k8s.BatchV1().Jobs(namespace).List(context.TODO(), v1.ListOptions{
	})
	if jErr != nil {
		logger.Error("error", jErr)
	}
	now := time.Now()
	for _, item := range jList.Items {
		succeeded := item.Status.Succeeded
		if succeeded == 1 {
			completionTime := item.Status.CompletionTime
			duration := now.Sub(completionTime.Time).Minutes()
			fields := map[string]interface{}{
				"task":      "job-cleanup",
				"name":      item.Name,
				"succeeded": succeeded,
				"completed": fmt.Sprintf("%f minutes ago", duration),
			}
			logger.WithFields(fields).Info("check job status")
			if thresholdMinutes < duration {
				err := k8s.BatchV1().Jobs(namespace).Delete(context.TODO(), item.Name, v1.DeleteOptions{})
				resultLog := logger.WithFields(fields).WithField("action", "clean")
				if err != nil {
					resultLog.WithField("delete", "FAIL").Info("clean completed jobs failed")
				} else {
					resultLog.WithField("delete", "SUCCESS").Info("clean completed jobs succeeded")
				}
			}
		}
	}
}
