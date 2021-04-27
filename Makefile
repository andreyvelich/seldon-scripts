deploy:
	bash scripts/deploy.sh

undeploy:
	kind delete cluster
