# Multi-stage build to extract files from official AWS Lambda runtime images
# This Dockerfile only depends on official AWS images and the RIE binary

# ===========================================
# Python runtime sources
# ===========================================
FROM public.ecr.aws/lambda/python:3.10 AS python310
FROM public.ecr.aws/lambda/python:3.11 AS python311
FROM public.ecr.aws/lambda/python:3.12 AS python312
FROM public.ecr.aws/lambda/python:3.13 AS python313
FROM public.ecr.aws/lambda/python:3.14 AS python314

# ===========================================
# Node.js runtime sources
# ===========================================
FROM public.ecr.aws/lambda/nodejs:20 AS nodejs20
FROM public.ecr.aws/lambda/nodejs:22 AS nodejs22
FROM public.ecr.aws/lambda/nodejs:24 AS nodejs24

# ===========================================
# Final stage - Python 3.12 base
# ===========================================
FROM public.ecr.aws/lambda/python:3.12

# ===========================================
# Copy Python 3.10 runtime
# Python 3.10 has libraries in /var/runtime (awslambdaric, boto3, etc.)
# ===========================================
COPY --from=python310 /var/runtime/bootstrap /var/runtime/python3.10-bootstrap
COPY --from=python310 /var/runtime/bootstrap.py /var/runtime/python3.10-bootstrap.py
COPY --from=python310 /var/lang /var/lang/python3.10
# Copy all Python 3.10 runtime libraries from python310's /var/runtime to site-packages
# Python 3.10 stores user libraries in /var/runtime, not /var/lang like newer versions
RUN mkdir -p /var/lang/python3.10/lib/python3.10/site-packages
COPY --from=python310 /var/runtime/ /var/lang/python3.10/lib/python3.10/site-packages/
# Remove bootstrap files from site-packages (they're already in /var/runtime)
RUN rm -f /var/lang/python3.10/lib/python3.10/site-packages/bootstrap \
           /var/lang/python3.10/lib/python3.10/site-packages/bootstrap.py

# ===========================================
# Copy Python 3.11 runtime
# ===========================================
COPY --from=python311 /var/runtime/bootstrap /var/runtime/python3.11-bootstrap
COPY --from=python311 /var/runtime/bootstrap.py /var/runtime/python3.11-bootstrap.py
COPY --from=python311 /var/lang /var/lang/python3.11

# ===========================================
# Python 3.12 is the base image
# ===========================================
COPY --from=python312 /var/runtime/bootstrap /var/runtime/python3.12-bootstrap
COPY --from=python312 /var/runtime/bootstrap.py /var/runtime/python3.12-bootstrap.py

# ===========================================
# Copy Python 3.13 runtime
# ===========================================
COPY --from=python313 /var/runtime/bootstrap /var/runtime/python3.13-bootstrap
COPY --from=python313 /var/runtime/bootstrap.py /var/runtime/python3.13-bootstrap.py
COPY --from=python313 /var/lang /var/lang/python3.13

# ===========================================
# Copy Python 3.14 runtime
# ===========================================
COPY --from=python314 /var/runtime/bootstrap /var/runtime/python3.14-bootstrap
COPY --from=python314 /var/runtime/bootstrap.py /var/runtime/python3.14-bootstrap.py
COPY --from=python314 /var/lang /var/lang/python3.14

# ===========================================
# Copy Node.js runtimes
# ===========================================
COPY --from=nodejs20 /var/runtime /var/runtime/nodejs20
COPY --from=nodejs20 /var/lang /var/lang/nodejs20
COPY --from=nodejs22 /var/runtime /var/runtime/nodejs22
COPY --from=nodejs22 /var/lang /var/lang/nodejs22
COPY --from=nodejs24 /var/runtime /var/runtime/nodejs24
COPY --from=nodejs24 /var/lang /var/lang/nodejs24

# Fix bootstrap scripts to use local index.mjs instead of /var/runtime/index.mjs
RUN sed -i 's|/var/runtime/index\.mjs|/var/runtime/nodejs20/index.mjs|g' /var/runtime/nodejs20/bootstrap
RUN sed -i 's|/var/runtime/index\.mjs|/var/runtime/nodejs22/index.mjs|g' /var/runtime/nodejs22/bootstrap
RUN sed -i 's|/var/runtime/index\.mjs|/var/runtime/nodejs24/index.mjs|g' /var/runtime/nodejs24/bootstrap
# Fix bootstrap scripts to use local node binary
RUN sed -i 's|/var/lang/bin/node|/var/lang/nodejs20/bin/node|g' /var/runtime/nodejs20/bootstrap
RUN sed -i 's|/var/lang/bin/node|/var/lang/nodejs22/bin/node|g' /var/runtime/nodejs22/bootstrap
RUN sed -i 's|/var/lang/bin/node|/var/lang/nodejs24/bin/node|g' /var/runtime/nodejs24/bootstrap

# ===========================================
# Copy RIE binary from ./bin/
# ===========================================
COPY bin/aws-lambda-rie /var/runtime/aws-lambda-rie
RUN chmod +x /var/runtime/aws-lambda-rie

# ===========================================
# Create multi-runtime bootstrap wrapper
# ===========================================
RUN mv /var/runtime/bootstrap /var/runtime/bootstrap.orig || true

RUN printf '#!/bin/bash\n\
RUNTIME="${AWS_LAMBDA_FUNCTION_RUNTIME}"\n\
HANDLER="${AWS_LAMBDA_FUNCTION_HANDLER:-index.handler}"\n\
echo "Bootstrap: runtime=$RUNTIME, handler=$HANDLER"\n\
case "$RUNTIME" in\n\
    nodejs20.x)\n\
        export LD_LIBRARY_PATH=/var/lang/nodejs20/lib:$LD_LIBRARY_PATH\n\
        exec /var/runtime/nodejs20/bootstrap "$HANDLER" ;;\n\
    nodejs22.x)\n\
        export LD_LIBRARY_PATH=/var/lang/nodejs22/lib:$LD_LIBRARY_PATH\n\
        exec /var/runtime/nodejs22/bootstrap "$HANDLER" ;;\n\
    nodejs24.x)\n\
        export LD_LIBRARY_PATH=/var/lang/nodejs24/lib:$LD_LIBRARY_PATH\n\
        exec /var/runtime/nodejs24/bootstrap "$HANDLER" ;;\n\
    python3.10)\n\
        export AWS_EXECUTION_ENV=AWS_Lambda_python3.10\n\
        export PYTHONHOME=/var/lang/python3.10\n\
        export PYTHONPATH=/var/lang/python3.10/lib/python3.10/site-packages:/var/task\n\
        export LD_LIBRARY_PATH=/var/lang/python3.10/lib:$LD_LIBRARY_PATH\n\
        exec /var/lang/python3.10/bin/python3.10 /var/runtime/python3.10-bootstrap.py "$@" ;;\n\
    python3.11)\n\
        export AWS_EXECUTION_ENV=AWS_Lambda_python3.11\n\
        export PYTHONHOME=/var/lang/python3.11\n\
        export PYTHONPATH=/var/lang/python3.11/lib/python3.11/site-packages:/var/task\n\
        export LD_LIBRARY_PATH=/var/lang/python3.11/lib:$LD_LIBRARY_PATH\n\
        exec /var/lang/python3.11/bin/python3.11 /var/runtime/python3.11-bootstrap.py "$@" ;;\n\
    python3.12)\n\
        export AWS_EXECUTION_ENV=AWS_Lambda_python3.12\n\
        export PYTHONPATH=/var/lang/lib/python3.12/site-packages:/var/task\n\
        exec /var/lang/bin/python3.12 /var/runtime/python3.12-bootstrap.py "$@" ;;\n\
    python3.13)\n\
        export AWS_EXECUTION_ENV=AWS_Lambda_python3.13\n\
        export PYTHONHOME=/var/lang/python3.13\n\
        export PYTHONPATH=/var/lang/python3.13/lib/python3.13/site-packages:/var/task\n\
        export LD_LIBRARY_PATH=/var/lang/python3.13/lib:$LD_LIBRARY_PATH\n\
        exec /var/lang/python3.13/bin/python3.13 /var/runtime/python3.13-bootstrap.py "$@" ;;\n\
    python3.14)\n\
        export AWS_EXECUTION_ENV=AWS_Lambda_python3.14\n\
        export PYTHONHOME=/var/lang/python3.14\n\
        export PYTHONPATH=/var/lang/python3.14/lib/python3.14/site-packages:/var/task\n\
        export LD_LIBRARY_PATH=/var/lang/python3.14/lib:$LD_LIBRARY_PATH\n\
        exec /var/lang/python3.14/bin/python3.14 /var/runtime/python3.14-bootstrap.py "$@" ;;\n\
    *)\n\
        echo "Error: Unsupported runtime: $RUNTIME"\n\
        exit 1 ;;\nesac' > /var/runtime/bootstrap && chmod +x /var/runtime/bootstrap

# ===========================================
# Copy entrypoint
# ===========================================
COPY lambda-entrypoint.sh /lambda-entrypoint.sh
RUN chmod +x /lambda-entrypoint.sh

# Do NOT set ENTRYPOINT - the container's Cmd will be set when creating containers