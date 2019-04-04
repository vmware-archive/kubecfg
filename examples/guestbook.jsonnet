// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Simple example to demonstrate kubecfg.

// This is a simple port to jsonnet of the standard guestbook example
// https://github.com/kubernetes/kubernetes/tree/master/examples/guestbook
//
// ```
// kubecfg update guestbook.jsonnet
// # poke at $(minikube service --url frontend), etc
// kubecfg delete guestbook.jsonnet
// ```

// This example uses kube.libsonnet from Bitnami.  There are other
// Kubernetes libraries available, or write your own!
local kube = import "https://github.com/bitnami-labs/kube-libsonnet/raw/52ba963ca44f7a4960aeae9ee0fbee44726e481f/kube.libsonnet";

// A function that returns 2 k8s objects: a redis Deployment and Service
local redis(name) = {
  svc: kube.Service(name) {
    // "$" is an alias for the result value from the current function/file
    target_pod:: $.deploy.spec.template,
  },

  deploy: kube.Deployment(name) {
    spec+: {
      template+: {
        spec+: {
          containers_: {
            redis: kube.Container("redis") {
              image: "bitnami/redis:4.0.9",
              resources: {requests: {cpu: "100m", memory: "100Mi"}},
              ports: [{containerPort: 6379}],

              // kube.libsonnet has a few optional "underscore"
              // helpers to convert k8s API structures into more
              // natural jsonnet structures.  See kube.libsonnet.
              env_: {
                REDIS_REPLICATION_MODE: "master",
                ALLOW_EMPTY_PASSWORD: "yes",
              },
            },
          },
        },
      },
    },
  },
};

// Note the jsonnet file evaluates to the last object in the file.
// Kubecfg expects this object to be a possibly nested collection
// (array or object) of Kubernetes API objects.
{
  frontend: {
    svc: kube.Service("frontend") {
      target_pod: $.frontend.deploy.spec.template,
      spec+: {type: "LoadBalancer"},
    },

    deploy: kube.Deployment("frontend") {
      spec+: {
        replicas: 3,
        template+: {
          spec+: {
            containers_+: {
              frontend: kube.Container("php-redis") {
                image: "gcr.io/google-samples/gb-frontend:v3",
                resources: {
                  requests: {cpu: "100m", memory: "100Mi"},
                },
                ports: [{containerPort: 80}],
                readinessProbe: {
                  httpGet: {path: "/", port: 80},
                },
                livenessProbe: self.readinessProbe {
                  initialDelaySeconds: 10,
                },
              },
            },
          },
        },
      },
    },
  },

  master: redis("redis-master"),

  // "Override" some parameters in the slave redis Deployment.
  slave: redis("redis-slave") {
    deploy+: {
      spec+: {
        replicas: 2,
        template+: {
          spec+: {
            containers_+: {
              redis+: {
                env_+: {
                  REDIS_REPLICATION_MODE: "slave",
                  REDIS_MASTER_HOST: $.master.svc.metadata.name,
                  REDIS_MASTER_PORT_NUMBER: "6379",
                },
              },
            },
          },
        },
      },
    },
  },
}
