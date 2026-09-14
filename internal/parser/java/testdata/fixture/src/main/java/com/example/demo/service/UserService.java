package com.example.demo.service;

import com.example.demo.model.User;
import com.example.demo.repository.UserRepository;

@Service
public class UserService {
    @Autowired
    private UserRepository userRepository;

    public User getUser(Long id) {
        return this.userRepository.findById(id);
    }

    public User loadUser(Long id) {
        return getUser(id);
    }
}
