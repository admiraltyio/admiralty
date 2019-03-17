import socket

import flask

app = flask.Flask(__name__)


@app.route('/')
def hello_world():
    return socket.gethostname()
