package com.example.demo.model;

@Table(name = "notes")
public class Note {
    private String text;

    public String getText() {
        return this.text;
    }
}
