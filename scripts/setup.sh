#!/usr/bin/env sh

set -e

echo "applying kafka resources..."

kubectl apply -f k8s/01-namespace
kubectl apply -f k8s/02-zookeeper
kubectl apply -f k8s/03-kafka

kubectl config set-context --current --namespace=kafka-ca1

echo "setting minikube for local docker images..."
eval "$(minikube docker-env --shell sh)"

echo "building docker images..."
(cd event-bus-listener && docker build -t event-bus-listener -f Dockerfile .)
(cd event-bus-publisher && docker build -t event-bus-publisher -f Dockerfile .)
(cd acceptance && docker build -t acceptance -f Dockerfile .)

echo "deploying apps..."
(cd event-bus-listener && kubectl apply -f .)
(cd event-bus-publisher && kubectl apply -f .)

# wait for zookeeper deployment and start acceptance job
kubectl rollout status deployment zookeeper
(cd acceptance && kubectl apply -f .)

run_job() {
  pod=""
  attempts=1
  max_attempts=15

  while [ -z "$pod" ]; do
      if [ "$attempts" -eq "$max_attempts" ]; then
        echo "failed to get job's pod"
        exit 1
      else
        echo "waiting for pod for job: acceptance-test-job : attempt $attempts"
        pod=$(kubectl get pods --selector=job-name="acceptance-test-job" --output=jsonpath='{.items..metadata.name}')
        attempts=$((attempts + 1))
        [ -z "$pod" ] && sleep 5
      fi
  done

  # hack, as i found no way to check that Job is `running` or `ready`
  sleep 10s

  echo "Job running on pod: $pod"
  kubectl logs -f "$pod"
}

run_job

echo "See You Space Cowboy..."
