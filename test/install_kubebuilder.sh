set -eu

version=1.0.7
arch=amd64
curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_linux_${arch}.tar.gz"
tar -zxvf kubebuilder_${version}_linux_${arch}.tar.gz
rm kubebuilder_${version}_linux_${arch}.tar.gz
mv kubebuilder_${version}_linux_${arch} /kubebuilder
