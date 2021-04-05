export VK_ROOT="$PWD"/..
echo "$VK_ROOT"
"$VK_ROOT"/_output/bin/helm template "$VK_ROOT"/installer/helm/chart/volcano --namespace volcano-system \
      --name volcano --set basic.image_tag_version=1ddb6f067429c7acca20affc47dfa62ccb08ecd7 --set basic.controller_image_name=yzs981130/vc-controller-manager --set basic.scheduler_image_name=yzs981130/vc-scheduler --set basic.admission_image_name=yzs981130/vc-webhook-manager --set basic.scheduler_config_file="$VK_ROOT"/script/volcano-scheduler.conf \
      -x templates/admission.yaml \
      -x templates/controllers.yaml \
      -x templates/scheduler.yaml \
      --notes > "test.yaml"
