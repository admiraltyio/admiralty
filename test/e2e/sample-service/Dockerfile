FROM python:3-alpine
RUN apk --no-cache add curl
RUN pip install --no-cache-dir pipenv
COPY Pipfile Pipfile.lock ./
RUN pipenv install --system --deploy
COPY app.py ./
ENV FLASK_APP=app.py
ENV FLASK_ENV=development
ENTRYPOINT [ "flask", "run", "--host", "0.0.0.0", "--port", "80" ]
