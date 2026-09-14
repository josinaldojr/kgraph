from flask import Flask
from .service import UserService

app = Flask(__name__)


@app.route('/users', methods=['GET'])
def get_users():
    return []


@app.get('/users/{user_id}')
def get_user(user_id):
    return {}


@app.post('/users')
def create_user():
    return {}


@app.route(
    '/users/<int:user_id>',
    methods=['DELETE'],
)
def delete_user(user_id):
    return {}
