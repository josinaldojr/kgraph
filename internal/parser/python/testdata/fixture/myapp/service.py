from .models import User


class UserService:
    def get_user(self, user_id):
        return self.load_user(user_id)

    def load_user(self, user_id):
        return User()


def format_user(user):
    return user.name
