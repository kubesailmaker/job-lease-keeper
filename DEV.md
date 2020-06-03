#### Setup
``` 
kubectl create ns operators
kubectl create ns broker
```

#### Create new service account
```

kubectl apply -n operators -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: job-lease-keeper
EOF
```
#### Create role in Default Namespace
```
kubectl apply -n default -f deploy/job_role.yaml 
```

#### Create RoleBinding for Operator
```
kubectl apply -n default -f -<<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: job-lease-keeper-broker
subjects:
  - kind: ServiceAccount
    name: job-lease-keeper
    namespace: operators
roleRef:
  kind: Role
  name: job-lease-keeper
  apiGroup: rbac.authorization.k8s.io
EOF
```

#### Create Yet another Role in a Namespace
```
kubectl apply -n broker -f deploy/job_role.yaml
```

#### Create RoleBinding for Operator in Another Namespace
```
kubectl apply -n broker -f -<<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: job-lease-keeper-broker
subjects:
  - kind: ServiceAccount
    name: job-lease-keeper
    namespace: operators
roleRef:
  kind: Role
  name: job-lease-keeper
  apiGroup: rbac.authorization.k8s.io
EOF

```
#### Deploy Job Cleaner to clean multiple namespaces

```
kubectl apply -n operators -f -<<EOF
apiVersion: extensions/v1beta1 
kind: Deployment
metadata:
  name: job-lease-keeper
spec:
  replicas: 1
  selector:
    matchLabels:
      name: job-lease-keeper
  template:
    metadata:
      labels:
        name: job-lease-keeper
    spec:
      serviceAccountName: job-lease-keeper
      containers:
       - name: job-lease-keeper
         image: kubesailmaker/job-lease-keeper:0.10.2
         command:
         - "/job-lease-keeper"
         imagePullPolicy: IfNotPresent
         env:
         - name: CONTROLLER_NAME
           valueFrom:
             fieldRef:
               fieldPath: metadata.name
         - name: JOBS_NAMESPACE
           value: "default,broker"               
         - name: JOBS_SUCCESS_THRESHOLD_MINUTES
           value: "10"
         - name: JOBS_FAILURE_THRESHOLD_MINUTES
           value: "20"
         - name: CHECK_FREQUENCY_MINUTES
           value: "5"
         - name: KUBERNETES_SERVICE_HOST
           value: "kubernetes.default.svc"
         - name: KUBERNETES_SERVICE_PORT
           value: "443"
         resources:
           requests:
             cpu: 50m
             memory: 256Mi
           limits:
             cpu: 50m
             memory: 256Mi
EOF
```

#### Add sample jobs in various namespaces

```
kubectl apply -f sample/job1.yaml -n broker
kubectl apply -f sample/job2.yaml -n broker
kubectl apply -f sample/job1.yaml -n default
kubectl apply -f sample/job2.yaml -n default
```

#### Cleanup
```
kubectl delete ns operators
kubectl delete ns broker
kubectl delete job --all -n default
 
```