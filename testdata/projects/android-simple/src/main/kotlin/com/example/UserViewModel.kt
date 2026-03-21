package com.example

import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.MutableStateFlow

class UserViewModel(
    private val userRepository: UserRepository
) {
    private val _state = MutableStateFlow(UserState())
    val state: StateFlow<UserState> = _state

    fun loadUser(id: String) {
        val user = userRepository.getUser(id)
        _state.value = UserState(user = user)
    }
}

data class UserState(val user: User? = null)
