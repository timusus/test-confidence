package com.example

import org.junit.Test
import io.mockk.every
import io.mockk.verify
import io.mockk.mockk
import io.mockk.MockK
import org.junit.Before
import org.junit.Ignore

class UserViewModelTest {

    @MockK
    lateinit var repository: UserRepository

    private lateinit var viewModel: UserViewModel

    @Before
    fun setUp() {
        repository = mockk()
        viewModel = UserViewModel(repository)
    }

    @Test
    fun `should load users`() {
        every { repository.getUsers() } returns listOf(testUser)
        val result = viewModel.loadUsers()
        assertThat(result).hasSize(1)
    }

    @Test
    @Ignore
    fun `should handle empty list`() {
    }

    @Test
    fun `should save user`() {
        viewModel.saveUser(testUser)
        verify { repository.save(testUser) }
    }
}
