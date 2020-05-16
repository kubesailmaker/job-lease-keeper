FROM golang:1.14 as builder
LABEL maintainers=kube-sailmaker target=kubernetes task="job cleanup"

RUN mkdir /build
WORKDIR /build
COPY . /build
RUN go mod vendor
RUN go build -o job-lease-keeper

FROM scratch
ENV JOBS_NAMESPACE default
ENV JOBS_SUCCESS_THRESHOLD_MINUTES 60
ENV JOBS_FAILURE_THRESHOLD_MINUTES 120
ENV CHECK_FREQUENCY_MINUTES 30

COPY --from=builder /build/job-lease-keeper /job-lease-keeper
CMD ["/job-lease-keeper"]

