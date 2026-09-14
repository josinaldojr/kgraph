package com.example.demo.controller;

import com.example.demo.model.User;
import com.example.demo.service.UserService;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/users")
public class UserController {
    @Autowired
    private UserService userService;

    @GetMapping("/{id}")
    public User getUser(Long id) {
        return this.userService.getUser(id);
    }

    @DeleteMapping(
        "/{id}"
    )
    public void deleteUser(Long id) {
    }
}
