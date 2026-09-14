@Entity('ts_users')
export class User {
    @Column()
    id: number;

    @Column('user_name')
    name: string;
}
