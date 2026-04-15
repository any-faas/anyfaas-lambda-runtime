when build the image:

1. build for local test: anyfaas-lambda-multi:latest
2. tag and push to aliyun: registry.cn-shanghai.aliyuncs.com/anyfaas/lambda-multi:{latest,prod}
3. tag and push to aws: public.ecr.aws/v4p2v0p0/anyfaas-lambda-multi:{latest,prod}
4. the credentials can be found at .env.sh

