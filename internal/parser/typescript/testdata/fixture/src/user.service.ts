import { User } from './user.entity';
import { UserRepository } from './user.repository';
import { Injectable } from '@nestjs/core';

@Injectable()
export class UserService {
    @Inject()
    private userRepository: UserRepository;

    getUser(id: number): User {
        return this.userRepository.findById(id);
    }

    loadUser(id: number): User {
        return this.getUser(id);
    }
}
