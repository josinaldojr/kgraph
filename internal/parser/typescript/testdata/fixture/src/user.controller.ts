import { User } from './user.entity';
import { UserService } from './user.service';

@Controller('users')
export class UserController {
    @Inject()
    private userService: UserService;

    @Get(':id')
    getUser(id: number): User {
        return this.userService.getUser(id);
    }

    @Post()
    createUser(): User {
        return this.userService.getUser(0);
    }

    @Delete(
        ':id'
    )
    deleteUser(id: number): void {
    }
}
