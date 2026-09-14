package com.example.demo.model;

@Entity
@Table(name = "users")
public class User {
    @Id
    @GeneratedValue
    private Long id;

    @Column(name = "user_name")
    private String name;

    public Long getId() {
        return this.id;
    }

    public String getName() {
        return this.name;
    }
}
