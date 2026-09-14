import { User } from './user.entity';

export interface UserRepository {
    findById(id: number): User;
}
