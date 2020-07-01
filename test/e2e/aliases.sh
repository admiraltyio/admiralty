#!/usr/bin/env bash

k() { KUBECONFIG=kubeconfig-cluster$1 kubectl "${@:2}"; }
h() { KUBECONFIG=kubeconfig-cluster$1 helm "${@:2}"; }
wk() { KUBECONFIG=kubeconfig-cluster$1 watch kubectl "${@:2}"; }
