#!/usr/bin/env bash

k() { KUBECONFIG=kubeconfig-cluster$1 kubectl "${@:2}"; }
h() { KUBECONFIG=kubeconfig-cluster$1 helm "${@:2}"; }
