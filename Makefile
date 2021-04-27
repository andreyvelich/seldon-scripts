deploy:
	bash deploy.sh

undeploy:
	kind delete cluster
