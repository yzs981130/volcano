_output/bin/vc-scheduler --log_dir=./log/ \
        --kubeconfig=/Users/gaowei/.kube/config \
        --scheduler-conf=./scripts/volcano-scheduler.conf \
        --schedule-period=5s \
        --v=4 