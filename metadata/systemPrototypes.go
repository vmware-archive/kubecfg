package metadata

import "github.com/ksonnet/kubecfg/metadata/prototype"

var defaultPrototypes = []*prototype.Specification{
	&prototype.Specification{
		APIVersion: "0.1",
		Name:       "io.ksonnet.pkg.simple-service",
		Params: prototype.Params{
			prototype.RequiredParam("name", "serviceName", "Name of the service"),
			prototype.OptionalParam("port", "servicePort", "Port to expose", "80"),
			prototype.OptionalParam("portName", "portName", "Name of the port to expose", "http"),
		},
		Template: prototype.Snippet{
			Description: "Generates a simple service with a port exposed",
			Body: []string{
				"local k = import 'ksonnet.beta.2/k.libsonnet';",
				"",
				"local service = k.core.v1.service;",
				"local servicePort = k.core.v1.service.mixin.spec.portsType;",
				"local port = servicePort.new(std.extVar('port'), std.extVar('portName'));",
				"",
				"local name = std.extVar('name');",
				"k.core.v1.service.new('%-service' % name, {app: name}, port)",
			},
		},
	},
	&prototype.Specification{
		APIVersion: "0.1",
		Name:       "io.ksonnet.pkg.simple-deployment",
		Params: prototype.Params{
			prototype.RequiredParam("name", "deploymentName", "Name of the deployment"),
			prototype.RequiredParam("image", "containerImage", "Container image to deploy"),
			prototype.OptionalParam("replicas", "replicas", "Number of replicas", "1"),
			prototype.OptionalParam("port", "containerPort", "Port to expose", "80"),
			prototype.OptionalParam("portName", "portName", "Name of the port to expose", "http"),
		},
		Template: prototype.Snippet{
			Description: `Instantiates a simple deployment. Pod is replicated 1 time by
default, and pod template labels are automatically populated
from the deployment name.`,
			Body: []string{
				"local k = import 'ksonnet.beta.2/k.libsonnet';",
				"local deployment = k.apps.v1beta1.deployment;",
				"local container = deployment.mixin.spec.template.spec.containersType;",
				"",
				"local appName = std.extVar('name');",
				"local appContainer = container.new(appName, std.extVar('image'));",
				"deployment.new(appName, std.extVar('replicas'), appContainer, {app: appName})",
			},
		},
	},
}
