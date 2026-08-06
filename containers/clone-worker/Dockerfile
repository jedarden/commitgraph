# clone-worker — AI coding tool detection. Built by the commitgraph-build
# WorkflowTemplate with a REPO-ROOT kaniko context and
# --dockerfile=containers/clone-worker/Dockerfile, so COPY paths
# are repo-root-relative.
FROM python:3.12-slim

WORKDIR /app

# Copy the detection module (no external dependencies - uses only stdlib)
COPY containers/clone-worker/detection.py /app/detection.py

# Verify the module is importable
RUN python3 -c "import detection; print('Detection module loaded successfully')"

CMD ["python3", "-u", "-c", "import detection; print('clone-worker detection module ready')"]
